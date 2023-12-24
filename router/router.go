package router

import (
	"log"
	"runtime"

	"github.com/gorilla/mux"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
	"github.com/prest/prest/middlewares"
	"github.com/prest/prest/plugins"
	"github.com/urfave/negroni/v3"
)

// GetRouter reagister all routes
// v2: this is not used anywhere, so we can make it private
func (r *Config) Get() {
	// TODO: allow logger customization
	routes := controllers.New(r.serverConfig, log.Default())

	r.router = mux.NewRouter().StrictSlash(true)

	if r.serverConfig.AuthEnabled {
		// can be db specific in the future, there's bellow a proposal
		// maybe disable on multiple databases
		r.router.HandleFunc("/auth", routes.Auth).Methods("POST")
		// multiple DB suggestion:
		// router.HandleFunc("/db/{database}/auth", routes.Auth).Methods("POST")
	}

	r.router.HandleFunc("/databases", routes.GetDatabases).Methods("GET")
	r.router.HandleFunc("/schemas", routes.GetSchemas).Methods("GET")
	r.router.HandleFunc("/tables", routes.GetTables).Methods("GET")

	// v2: add this route to the router
	// breaking change
	r.router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", routes.ExecuteFromScripts)
	// r.router.HandleFunc("/_QUERIES/{database}/{queriesLocation}/{script}", routes.ExecuteFromScripts)

	// if it is windows it should not register the plugin endpoint
	// we use go plugin system that does not support windows
	// https://github.com/golang/go/issues/19282
	if runtime.GOOS != "windows" {
		r.router.HandleFunc("/_PLUGIN/{file}/{func}", plugins.HandlerPlugin)
	}

	r.router.HandleFunc("/{database}/{schema}", routes.GetTablesByDatabaseAndSchema).Methods("GET")
	r.router.HandleFunc("/show/{database}/{schema}/{table}", routes.ShowTable).Methods("GET")

	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	r.router.HandleFunc("/_health", controllers.WrappedHealthCheck(controllers.DefaultCheckList)).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", routes.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", routes.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", routes.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", routes.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", routes.UpdateTable).Methods("PUT", "PATCH")

	r.router.PathPrefix("/").Handler(
		negroni.New(
			middlewares.ExposureMiddleware(r.serverConfig),
			middlewares.AccessControl(r.serverConfig),
			middlewares.AuthMiddleware(r.serverConfig),
			middlewares.CacheMiddleware(r.serverConfig),
			// plugins middleware
			plugins.MiddlewarePlugin(r.serverConfig.PluginPath, r.serverConfig.PluginMiddlewareList),
			negroni.Wrap(crudRoutes),
		),
	)
}

// Routes for pREST
func Routes(cfg *config.Prest) *negroni.Negroni {
	n := middlewares.GetApp(cfg)
	n.UseHandler(New(cfg).router)
	return n
}
