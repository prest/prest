package app

import (
	"errors"
	"net/http"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
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

	deps := controllers.NewDepsFromConfig(cfg)
	h := controllers.NewHandlers(deps)

	plg := plugins.New(cfg)
	crud := middlewares.NewCRUDStack(cfg, plg)

	mux := mux.NewRouter().StrictSlash(true)
	router.RegisterRoutes(mux, cfg, h, crud, plg)

	n := middlewares.New(cfg)
	n.UseHandler(mux)
	return &App{Config: cfg, Handler: n, pg: cfg.Adapter}, nil
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
