package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mysql"
	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/adapters/timescaledb"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/internal/logsafe"
	"github.com/prest/prest/v2/middlewares"
	"github.com/prest/prest/v2/plugins"
	"github.com/prest/prest/v2/router"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// App is the composition root for the HTTP server.
type App struct {
	Config   *config.Prest
	Handler  http.Handler
	Adapters adapters.Registry
	pg       adapters.Adapter // deprecated: kept for backward compatibility
}

// New builds a ready-to-serve App from cfg.
//
// Creates and registers adapters for each configured database. If cfg.Adapter is nil
// and no database registry is configured, detects and creates a default adapter.
// Handlers, CRUD middleware, routes, global middleware, and plugins are wired into
// a single http.Handler.
//
// In registry mode an entry that is unreachable at startup is served by a lazy
// adapter (it connects on first use) and skipped by startup migrations.
//
// Returns an error when the database connection cannot be established: in
// single-database mode on any failure, in registry mode on configuration
// errors or when no entry is reachable.
func New(cfg *config.Prest) (*App, error) {
	registry := adapters.NewRegistry()
	var unreachable map[string]struct{}

	// Multi-database mode: create adapter for each configured database
	if cfg.HasDatabaseRegistry() {
		var err error
		if unreachable, err = registerDatabases(cfg, registry); err != nil {
			return nil, err
		}
	} else if cfg.Adapter == nil {
		// Single database mode (backward compatibility): detect and create default adapter
		adapter, err := detectAndCreateAdapter(cfg)
		if err != nil {
			return nil, err
		}
		cfg.Adapter = adapter
		alias := cfg.PGDatabase
		if alias == "" {
			alias = "prest" // Use default alias when database name is not set
		}
		if err := registry.Register(alias, adapter); err != nil {
			return nil, err
		}
	} else {
		// Adapter already configured (injected): register it
		alias := cfg.PGDatabase
		if alias == "" {
			alias = "prest" // Use default alias when database name is not set
		}
		if err := registry.Register(alias, cfg.Adapter); err != nil {
			return nil, err
		}
	}

	logServedDatabases(cfg)

	if err := ensureSchemaMigrated(cfg, registry, unreachable); err != nil {
		return nil, err
	}

	if err := ensureQueriesImported(cfg); err != nil {
		return nil, err
	}

	// For multi-database mode, set a default adapter for health checks and schema operations
	if cfg.Adapter == nil {
		cfg.Adapter = defaultAdapter(cfg, registry, unreachable)
	}

	deps := controllers.NewDepsFromConfig(cfg)
	deps.AdapterRegistry = registry // Inject registry into deps
	deps.AdapterName = adapterEngineName(cfg.Adapter)
	h := controllers.NewHandlers(deps, cfg)

	plg := plugins.New(cfg)
	crud := middlewares.NewCRUDStack(cfg, plg)
	queryStack := middlewares.NewQueryStack(cfg, middlewares.ScriptPermsFromAdapter(cfg.Adapter))
	var adminStack *middlewares.AdminQueryStack
	if cfg.QueriesConf.RegisterEnabled && cfg.QueriesConf.Storage == config.QueriesStorageDatabase {
		adminStack = middlewares.NewAdminQueryStack(cfg)
	}

	mux := mux.NewRouter().StrictSlash(true)
	router.RegisterRoutes(mux, cfg, h, crud, queryStack, adminStack, plg)
	// After the match, so mux.Vars includes {database} before route middleware runs.
	mux.Use(func(next http.Handler) http.Handler {
		return middlewares.NewAdapterSelectorMiddleware(registry, next)
	})

	n := middlewares.New(cfg)
	n.UseHandler(mux)

	var handler http.Handler = n
	if cfg.Otel.Enabled {
		// Outermost span covers the whole middleware chain and extracts inbound
		// W3C trace context. Per-route http.route tags are added in the router.
		handler = otelhttp.NewHandler(handler, "prest")
	}
	return &App{Config: cfg, Handler: handler, Adapters: registry, pg: cfg.Adapter}, nil
}

// registerDatabases connects and registers an adapter for every registry
// entry. An unreachable entry gets a lazy adapter and is returned in the
// unreachable set; any other error is fatal, as is every entry being down.
func registerDatabases(cfg *config.Prest, registry adapters.Registry) (map[string]struct{}, error) {
	unreachable := make(map[string]struct{})
	for _, dbConf := range cfg.Databases {
		dbCfg := scopedConfig(cfg, &dbConf)
		adapter, err := connectAdapter(&dbCfg, &dbConf)
		if err != nil {
			// Auth failures, unknown engines and bad URLs are config mistakes, not outages.
			if !adapters.IsUnavailable(err) {
				return nil, err
			}
			slog.Warn("database unreachable at startup; serving lazily", "alias", dbConf.Alias, "err", logsafe.Error(err))
			adapter = lazyAdapterForDatabase(&dbCfg, &dbConf)
			unreachable[dbConf.Alias] = struct{}{}
		}
		if err := registry.Register(dbConf.Alias, adapter); err != nil {
			return nil, err
		}
		slog.Info("registered adapter for database", "alias", dbConf.Alias)
	}
	if len(unreachable) == len(cfg.Databases) {
		return nil, fmt.Errorf("no configured database reachable: %w", adapters.ErrUnavailable)
	}
	return unreachable, nil
}

// logServedDatabases tells operators which database names are reachable when
// pg.single=false runs without a registry: the configured database plus
// pg.allowed_databases. Every other name is rejected.
func logServedDatabases(cfg *config.Prest) {
	if cfg.SingleDB || cfg.HasDatabaseRegistry() {
		return
	}
	served := append([]string{cfg.PGDatabase}, cfg.AllowedDatabases...)
	slog.Info("pg.single=false: serving databases", "databases", served)
}

// defaultAdapter picks the adapter behind requests without {database}, health
// checks and script permissions: the first configured alias that was
// reachable at startup, else the first configured alias.
func defaultAdapter(cfg *config.Prest, registry adapters.Registry, unreachable map[string]struct{}) adapters.Adapter {
	if len(cfg.Databases) == 0 {
		return nil
	}
	alias := cfg.Databases[0].Alias
	for _, dbConf := range cfg.Databases {
		if _, down := unreachable[dbConf.Alias]; !down {
			alias = dbConf.Alias
			break
		}
	}
	adapter, err := registry.Get(alias)
	if err != nil {
		return nil
	}
	return adapter
}

// ensureSchemaMigrated runs the startup migrations on every registered alias
// except those in skip (unreachable at startup); they migrate on a later start.
func ensureSchemaMigrated(cfg *config.Prest, registry adapters.Registry, skip map[string]struct{}) error {
	needAuth := cfg.AuthEnabled && cfg.AuthMigrateOnStartup
	needQueries := cfg.QueriesConf.Storage == config.QueriesStorageDatabase && cfg.QueriesConf.MigrateOnStartup
	if !needAuth && !needQueries {
		return nil
	}

	if registry == nil {
		return nil
	}
	for _, alias := range registry.GetAll() {
		if _, down := skip[alias]; down {
			slog.Warn("auth/queries table migration skipped: database unreachable at startup", "alias", alias)
			continue
		}
		adapter, err := registry.Get(alias)
		if err != nil || adapter == nil {
			continue
		}
		if ensurer, ok := adapter.(adapters.SystemTableEnsurer); ok {
			if err := runSystemTables(ensurer, alias, needAuth, needQueries); err != nil {
				return err
			}
			continue
		}
		if engineForAlias(cfg, alias) != config.EnginePostgres {
			continue
		}
		db, err := postgres.DB(adapter)
		if err != nil {
			if errors.Is(err, postgres.ErrNotPostgresAdapter) {
				return fmt.Errorf("acquire database connection for startup migration: %w", ErrAdapterNotPostgres)
			}
			return fmt.Errorf("acquire database connection for startup migration: %w", err)
		}
		if needAuth {
			if err := EnsureAuthTable(cfg, db); err != nil {
				return fmt.Errorf("migrate auth table %s.%s: %w", cfg.AuthSchema, cfg.AuthTable, err)
			}
			slog.Info("auth table migration complete", "schema", cfg.AuthSchema, "table", cfg.AuthTable)
		}
		if needQueries {
			qc := cfg.QueriesConf
			if err := EnsureQueriesTable(cfg, db); err != nil {
				return fmt.Errorf("migrate queries table %s.%s: %w", qc.Schema, qc.Table, err)
			}
			slog.Info("queries table migration complete", "schema", qc.Schema, "table", qc.Table)
		}
	}
	return nil
}

// engineForAlias returns the engine for a registered alias.
// A matching Databases entry uses that entry's engine, otherwise the root engine.
func engineForAlias(cfg *config.Prest, alias string) string {
	if cfg != nil {
		for i := range cfg.Databases {
			if cfg.Databases[i].Alias == alias {
				return config.EffectiveEngine(&cfg.Databases[i], cfg)
			}
		}
	}
	return config.EffectiveEngine(nil, cfg)
}

func runSystemTables(ensurer adapters.SystemTableEnsurer, alias string, needAuth, needQueries bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if alias != "" {
		ctx = context.WithValue(ctx, pctx.DBNameKey, alias)
	}
	if needAuth {
		if err := ensurer.EnsureAuthTable(ctx); err != nil {
			return fmt.Errorf("migrate auth table: %w", err)
		}
		slog.Info("auth table migration complete", "alias", alias)
	}
	if needQueries {
		if err := ensurer.EnsureQueriesTable(ctx); err != nil {
			return fmt.Errorf("migrate queries table: %w", err)
		}
		slog.Info("queries table migration complete", "alias", alias)
	}
	return nil
}

func ensureQueriesImported(cfg *config.Prest) error {
	qc := cfg.QueriesConf
	if qc.Storage != config.QueriesStorageDatabase || !qc.ImportOnStartup {
		return nil
	}
	queriesPath := cfg.QueriesPath
	if env := os.Getenv("PREST_QUERIES_LOCATION"); env != "" {
		queriesPath = env
	}
	if queriesPath == "" {
		return nil
	}

	registry, ok := cfg.Adapter.(adapters.QueryRegistry)
	if !ok {
		return ErrAdapterNotQueryRegistry
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, pctx.DBNameKey, cfg.PGDatabase)

	report, err := registry.ImportFromFilesystem(ctx, queriesPath, qc.ImportPolicy)
	if err != nil {
		return fmt.Errorf("import query scripts from %s: %w", queriesPath, err)
	}
	slog.Info("queries filesystem import complete",
		"inserted", report.Inserted,
		"updated", report.Updated,
		"skipped", report.Skipped,
		"location", queriesPath)
	return nil
}

// EnsureAdapter connects the configured adapter when cfg.Adapter is nil.
func EnsureAdapter(cfg *config.Prest) error {
	if cfg.Adapter != nil {
		return nil
	}
	if config.EffectiveEngine(nil, cfg) == config.EngineMySQL {
		my := mysql.New(cfg)
		if err := mysql.Connect(my); err != nil {
			return err
		}
		cfg.Adapter = my
		return nil
	}
	pg := postgres.New(cfg)
	if err := postgres.Connect(pg); err != nil {
		return err
	}
	cfg.Adapter = pg
	return nil
}

// PostgresDB returns a sqlx connection from the configured postgres adapter.
func PostgresDB(cfg *config.Prest) (*sqlx.DB, error) {
	if err := EnsureAdapter(cfg); err != nil {
		return nil, err
	}
	db, err := postgres.DB(cfg.Adapter)
	if err != nil {
		if errors.Is(err, postgres.ErrNotPostgresAdapter) {
			return nil, ErrAdapterNotPostgres
		}
		return nil, err
	}
	return db, nil
}

// adapterEngineName reports the default adapter for /_health.
// A nil adapter and test doubles are reported as postgres.
func adapterEngineName(a adapters.Adapter) string {
	switch a.(type) {
	case *mysql.Adapter:
		return "mysql"
	case *timescaledb.Adapter:
		return "timescaledb"
	default:
		return "postgres"
	}
}

// detectAndCreateAdapter tries to connect to TimescaleDB first; if not available, falls back to PostgreSQL.
// This allows pREST to auto-detect and use the appropriate adapter without configuration.
func detectAndCreateAdapter(cfg *config.Prest) (adapters.Adapter, error) {
	if err := cfg.ValidateEngines(); err != nil {
		return nil, err
	}
	if config.EffectiveEngine(nil, cfg) == config.EngineMySQL {
		my := mysql.New(cfg)
		if err := mysql.Connect(my); err != nil {
			return nil, err
		}
		slog.Info("using mysql adapter")
		return my, nil
	}
	// Try TimescaleDB first
	tsAdapter := timescaledb.New(cfg)
	err := timescaledb.Connect(tsAdapter)
	if err == nil {
		slog.Info("detected TimescaleDB; using timescaledb adapter")
		return tsAdapter, nil
	}
	if adapters.IsUnavailable(err) {
		// Unreachable host: a Postgres retry would only wait for a second timeout.
		return nil, err
	}
	// Reachable but not TimescaleDB: fall back to PostgreSQL
	pgAdapter := postgres.New(cfg)
	if err := postgres.Connect(pgAdapter); err != nil {
		return nil, err
	}
	slog.Info("using postgres adapter")
	return pgAdapter, nil
}

// scopedConfig returns a copy of cfg scoped to one registry entry. Pool limits
// are copied only when set, so an entry never passes 0 (unlimited) to
// database/sql; cfg itself is not modified.
func scopedConfig(cfg *config.Prest, dbConf *config.DatabaseConf) config.Prest {
	eng := config.EffectiveEngine(dbConf, cfg)
	dbCfg := *cfg
	dbCfg.Engine = eng
	dbCfg.PGHost = dbConf.Host
	dbCfg.PGPort = dbConf.Port
	dbCfg.PGUser = dbConf.User
	dbCfg.PGPass = dbConf.Pass
	dbCfg.PGDatabase = dbConf.Database
	if dbConf.MaxOpenConn > 0 {
		dbCfg.PGMaxOpenConn = dbConf.MaxOpenConn
	}
	if dbConf.MaxIdleConn > 0 {
		dbCfg.PGMaxIdleConn = dbConf.MaxIdleConn
	}
	dbCfg.PGSSLMode = dbConf.SSL.Mode
	dbCfg.PGSSLCert = dbConf.SSL.Cert
	dbCfg.PGSSLKey = dbConf.SSL.Key
	dbCfg.PGSSLRootCert = dbConf.SSL.RootCert
	if dbConf.URL != "" && (eng != config.EngineMySQL || strings.HasPrefix(strings.ToLower(dbConf.URL), "mysql://")) {
		dbCfg.PGURL = dbConf.URL
	}
	if eng == config.EngineMySQL {
		config.ApplyMySQLRegistrySchemas(&dbCfg, dbConf.Database)
	}
	return dbCfg
}

// connectAdapter is the registry-mode connector; tests replace it to inject adapters.
var connectAdapter = connectAdapterForDatabase

// connectAdapterForDatabase creates and connects an adapter for one registry
// entry, using dbCfg from scopedConfig. Postgres entries are probed as
// TimescaleDB first; an unreachable host is reported without a Postgres retry.
func connectAdapterForDatabase(dbCfg *config.Prest, dbConf *config.DatabaseConf) (adapters.Adapter, error) {
	if dbCfg.Engine == config.EngineMySQL {
		my := mysql.New(dbCfg)
		if err := mysql.Connect(my); err != nil {
			return nil, fmt.Errorf("failed to connect to database %s: %w", dbConf.Alias, err)
		}
		my.SetDatabase(dbConf.Alias)
		slog.Info("using mysql adapter for database", "alias", dbConf.Alias)
		return my, nil
	}
	if dbCfg.Engine != config.EnginePostgres {
		return nil, fmt.Errorf("database %s: unknown database engine %q", dbConf.Alias, dbCfg.Engine)
	}

	// Try TimescaleDB first, fall back to PostgreSQL
	tsAdapter := timescaledb.New(dbCfg)
	err := timescaledb.Connect(tsAdapter)
	if err == nil {
		slog.Info("detected TimescaleDB for database", "alias", dbConf.Alias)
		return tsAdapter, nil
	}
	if adapters.IsUnavailable(err) {
		return nil, fmt.Errorf("failed to connect to database %s: %w", dbConf.Alias, err)
	}

	// Reachable but not TimescaleDB: fall back to PostgreSQL
	pgAdapter := postgres.New(dbCfg)
	if err := postgres.Connect(pgAdapter); err != nil {
		return nil, fmt.Errorf("failed to connect to database %s: %w", dbConf.Alias, err)
	}
	slog.Info("using postgres adapter for database", "alias", dbConf.Alias)
	return pgAdapter, nil
}

// lazyAdapterForDatabase builds an unconnected adapter of the entry's engine;
// its pool dials on first use, so requests recover once the database is up.
// TimescaleDB detection is skipped: hypertable endpoints for the alias stay
// unavailable until restart.
func lazyAdapterForDatabase(dbCfg *config.Prest, dbConf *config.DatabaseConf) adapters.Adapter {
	if dbCfg.Engine == config.EngineMySQL {
		my := mysql.New(dbCfg)
		my.SetDatabase(dbConf.Alias)
		return my
	}
	slog.Warn("timescaledb detection skipped: database unreachable at startup", "alias", dbConf.Alias)
	pg := postgres.New(dbCfg)
	// Same database selection postgres.Connect would have made.
	pg.SetDatabase(dbCfg.PGDatabase)
	return pg
}
