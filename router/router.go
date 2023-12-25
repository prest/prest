package router

import (
	"log"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"

	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
	"github.com/prest/prest/middlewares"
	"github.com/prest/prest/plugins"
)

// Routes gets pREST routes
func Routes(cfg *config.Prest) (*negroni.Negroni, error) {
	n := middlewares.Get(cfg)
	r, err := New(cfg)
	if err != nil {
		return nil, err
	}
	n.UseHandler(r.router)
	return n, nil
}

// ConfigRoutes reagister all handlers and routes
// v2: this is not used anywhere, so we can make it private
//
// todo: receive controller interface and mock handlers in tests
func (r *Config) ConfigRoutes() error {
	// TODO: allow logger customization
	handlers, err := controllers.New(r.srvCfg, log.Default())
	if err != nil {
		return err
	}

	r.router = mux.NewRouter().StrictSlash(true)

	if r.srvCfg.AuthEnabled {
		// can be db specific in the future, there's bellow a proposal
		// maybe disable on multiple databases
		r.router.HandleFunc("/auth", handlers.Auth).Methods("POST")
		// multiple DB suggestion:
		// router.HandleFunc("/db/{database}/auth", handlers.Auth).Methods("POST")
	}

	r.router.HandleFunc("/databases", handlers.GetDatabases).Methods("GET")
	r.router.HandleFunc("/schemas", handlers.GetSchemas).Methods("GET")
	r.router.HandleFunc("/tables", handlers.GetTables).Methods("GET")

	// v2: add this route to the router
	// breaking change
	r.router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", handlers.ExecuteFromScripts)
	// r.router.HandleFunc("/_QUERIES/{database}/{queriesLocation}/{script}", handlers.ExecuteFromScripts)

	// if it is windows it should not register the plugin endpoint
	// we use go plugin system that does not support windows
	// https://github.com/golang/go/issues/19282
	if runtime.GOOS != "windows" {
		r.router.HandleFunc("/_PLUGIN/{file}/{func}", plugins.HandlerPlugin)
	}

	r.router.HandleFunc("/{database}/{schema}", handlers.GetTablesByDatabaseAndSchema).Methods("GET")
	r.router.HandleFunc("/show/{database}/{schema}/{table}", handlers.ShowTable).Methods("GET")

	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	r.router.HandleFunc("/_health", controllers.WrappedHealthCheck(controllers.DefaultCheckList)).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", handlers.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", handlers.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", handlers.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", handlers.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", handlers.UpdateTable).Methods("PUT", "PATCH")

	r.router.PathPrefix("/").Handler(
		negroni.New(
			middlewares.ExposureMiddleware(&r.srvCfg.ExposeConf),
			middlewares.AccessControl(handlers.GetAdapter().TablePermissions),
			middlewares.AuthMiddleware(
				r.srvCfg.AuthEnabled, r.srvCfg.JWTKey, r.srvCfg.JWTWhiteList),
			middlewares.CacheMiddleware(r.srvCfg),
			// plugins middleware
			plugins.MiddlewarePlugin(r.srvCfg.PluginPath, r.srvCfg.PluginMiddlewareList),
			negroni.Wrap(crudRoutes),
		),
	)
	return nil
}
