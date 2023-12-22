package router

import (
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
func (r *Config) Get() *mux.Router {
	r.MuxRouter = mux.NewRouter().StrictSlash(true)
	if r.ServerConfig.AuthEnabled {
		// can be db specific in the future, there's bellow a proposal
		// maybe disable on multiple databases
		r.MuxRouter.HandleFunc("/auth", controllers.Auth).Methods("POST")
		// multiple DB suggestion:
		// router.HandleFunc("/db/{database}/auth", controllers.Auth).Methods("POST")
	}

	r.MuxRouter.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")
	r.MuxRouter.HandleFunc("/schemas", controllers.GetSchemas).Methods("GET")
	r.MuxRouter.HandleFunc("/tables", controllers.GetTables).Methods("GET")

	// breaking change
	r.MuxRouter.HandleFunc("/_QUERIES/{queriesLocation}/{script}", controllers.ExecuteFromScripts)
	// r.MuxRouter.HandleFunc("/_QUERIES/{database}/{queriesLocation}/{script}", controllers.ExecuteFromScripts)
	// if it is windows it should not register the plugin endpoint
	// we use go plugin system that does not support windows
	// https://github.com/golang/go/issues/19282
	if runtime.GOOS != "windows" {
		r.MuxRouter.HandleFunc("/_PLUGIN/{file}/{func}", plugins.HandlerPlugin)
	}
	r.MuxRouter.HandleFunc("/{database}/{schema}", controllers.GetTablesByDatabaseAndSchema).Methods("GET")
	r.MuxRouter.HandleFunc("/show/{database}/{schema}/{table}", controllers.ShowTable).Methods("GET")
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	r.MuxRouter.HandleFunc("/_health", controllers.WrappedHealthCheck(controllers.DefaultCheckList)).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", controllers.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.UpdateTable).Methods("PUT", "PATCH")
	r.MuxRouter.PathPrefix("/").Handler(negroni.New(
		middlewares.ExposureMiddleware(r.ServerConfig),
		middlewares.AccessControl(r.ServerConfig),
		middlewares.AuthMiddleware(r.ServerConfig),
		middlewares.CacheMiddleware(r.ServerConfig),
		// plugins middleware
		plugins.MiddlewarePlugin(),
		negroni.Wrap(crudRoutes),
	))

	return r.MuxRouter
}

// Routes for pREST
func Routes(cfg *config.Prest) *negroni.Negroni {
	n := middlewares.GetApp(cfg)
	n.UseHandler(NewRouter(cfg).Get())
	return n
}
