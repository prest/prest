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
	Config  *config.Prest
	Handler http.Handler
	pg      adapters.Adapter
}

// New builds a ready-to-serve App from cfg.
//
// If cfg.Adapter is nil, a postgres adapter is created and connected; the
// resulting adapter is stored back on cfg for reuse. Handlers, CRUD middleware,
// routes, global middleware, and plugins are wired into a single http.Handler.
//
// Returns an error when the database connection cannot be established.
func New(cfg *config.Prest) (*App, error) {
	if cfg.Adapter == nil {
		pg := postgres.New(cfg)
		if err := postgres.Connect(pg); err != nil {
			return nil, err
		}
		cfg.Adapter = pg
	}

	if err := ensureSchemaMigrated(cfg); err != nil {
		return nil, err
	}

	if err := ensureQueriesImported(cfg); err != nil {
		return nil, err
	}

	deps := controllers.NewDepsFromConfig(cfg)
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

	n := middlewares.New(cfg)
	n.UseHandler(mux)
	return &App{Config: cfg, Handler: n, pg: cfg.Adapter}, nil
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
