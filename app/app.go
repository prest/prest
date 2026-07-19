package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/adapters/timescaledb"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/middlewares"
	"github.com/prest/prest/v2/plugins"
	"github.com/prest/prest/v2/router"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
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
// Returns an error when the database connection cannot be established.
func New(cfg *config.Prest) (*App, error) {
	registry := adapters.NewRegistry()

	// Multi-database mode: create adapter for each configured database
	if cfg.HasDatabaseRegistry() {
		for _, dbConf := range cfg.Databases {
			adapter, err := createAdapterForDatabase(cfg, &dbConf)
			if err != nil {
				return nil, err
			}
			if err := registry.Register(dbConf.Alias, adapter); err != nil {
				return nil, err
			}
			slog.Info("registered adapter for database", "alias", dbConf.Alias)
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

	if err := ensureSchemaMigrated(cfg); err != nil {
		return nil, err
	}

	if err := ensureQueriesImported(cfg); err != nil {
		return nil, err
	}

	// For multi-database mode, set a default adapter for health checks and schema operations
	// Use the first registered adapter if no primary adapter is configured
	if cfg.Adapter == nil && len(registry.GetAll()) > 0 {
		aliases := registry.GetAll()
		defaultAdapter, _ := registry.Get(aliases[0])
		cfg.Adapter = defaultAdapter
	}

	deps := controllers.NewDepsFromConfig(cfg)
	deps.AdapterRegistry = registry // Inject registry into deps
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

	// Add adapter selector middleware for multi-database routing
	// This attaches the correct adapter to each request based on the database name in the URL
	muxWithAdapter := middlewares.NewAdapterSelectorMiddleware(registry, mux)

	n := middlewares.New(cfg)
	n.UseHandler(muxWithAdapter)
	return &App{Config: cfg, Handler: n, Adapters: registry, pg: cfg.Adapter}, nil
}

func ensureSchemaMigrated(cfg *config.Prest) error {
	needAuth := cfg.AuthEnabled && cfg.AuthMigrateOnStartup
	needQueries := cfg.QueriesConf.Storage == config.QueriesStorageDatabase && cfg.QueriesConf.MigrateOnStartup
	if !needAuth && !needQueries {
		return nil
	}

	db, err := PostgresDB(cfg)
	if err != nil {
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

// EnsureAdapter connects the postgres adapter when cfg.Adapter is nil.
func EnsureAdapter(cfg *config.Prest) error {
	if cfg.Adapter != nil {
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

// detectAndCreateAdapter tries to connect to TimescaleDB first; if not available, falls back to PostgreSQL.
// This allows pREST to auto-detect and use the appropriate adapter without configuration.
func detectAndCreateAdapter(cfg *config.Prest) (adapters.Adapter, error) {
	// Try TimescaleDB first
	tsAdapter := timescaledb.New(cfg)
	if err := timescaledb.Connect(tsAdapter); err == nil {
		slog.Info("detected TimescaleDB; using timescaledb adapter")
		return tsAdapter, nil
	}
	// Fallback to PostgreSQL
	pgAdapter := postgres.New(cfg)
	if err := postgres.Connect(pgAdapter); err != nil {
		return nil, err
	}
	slog.Info("using postgres adapter")
	return pgAdapter, nil
}

// createAdapterForDatabase creates and connects an adapter for a specific database configuration.
// Currently all databases use the postgres adapter (wire-compatible mode).
// In the future, this can route to TimescaleDB, MySQL, or other adapters based on detection.
func createAdapterForDatabase(cfg *config.Prest, dbConf *config.DatabaseConf) (adapters.Adapter, error) {
	// Create a temporary config scoped to this database for adapter creation
	dbCfg := *cfg
	dbCfg.PGHost = dbConf.Host
	dbCfg.PGPort = dbConf.Port
	dbCfg.PGUser = dbConf.User
	dbCfg.PGPass = dbConf.Pass
	dbCfg.PGDatabase = dbConf.Database
	dbCfg.PGMaxOpenConn = dbConf.MaxOpenConn
	dbCfg.PGMaxIdleConn = dbConf.MaxIdleConn
	dbCfg.PGSSLMode = dbConf.SSL.Mode
	dbCfg.PGSSLCert = dbConf.SSL.Cert
	dbCfg.PGSSLKey = dbConf.SSL.Key
	dbCfg.PGSSLRootCert = dbConf.SSL.RootCert
	if dbConf.URL != "" {
		dbCfg.PGURL = dbConf.URL
	}

	// Try TimescaleDB first, fall back to PostgreSQL
	tsAdapter := timescaledb.New(&dbCfg)
	if err := timescaledb.Connect(tsAdapter); err == nil {
		slog.Info("detected TimescaleDB for database", "alias", dbConf.Alias)
		return tsAdapter, nil
	}

	// Fallback to PostgreSQL
	pgAdapter := postgres.New(&dbCfg)
	if err := postgres.Connect(pgAdapter); err != nil {
		return nil, fmt.Errorf("failed to connect to database %s: %w", dbConf.Alias, err)
	}
	slog.Info("using postgres adapter for database", "alias", dbConf.Alias)
	return pgAdapter, nil
}
