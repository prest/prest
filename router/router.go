package router

import (
	"runtime"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"

	"github.com/prest/prest/cache"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
	"github.com/prest/prest/middlewares"
	"github.com/prest/prest/plugins"
)

// Routes gets pREST routes
func Routes(cfg *config.Prest, cacher cache.Cacher, pl plugins.Loader) (*negroni.Negroni, error) {
	n := middlewares.Get(cfg, cacher)
	r, err := New(cfg, cacher, pl)
	if err != nil {
		return nil, err
	}
	n.UseHandler(r.router)
	return n, nil
}

// ConfigRoutes reagister all handlers and routes
// v2: this is not used anywhere, so we can make it private
func (r *Config) ConfigRoutes(srv controllers.Server) error {

	r.router = mux.NewRouter().StrictSlash(true)

	if r.srvCfg.AuthEnabled {
		// can be db specific in the future, there's bellow a proposal
		// maybe disable on multiple databases
		r.router.HandleFunc("/auth", srv.Auth).Methods("POST")
		// multiple DB suggestion:
		// router.HandleFunc("/db/{database}/auth", srv.Auth).Methods("POST")
	}

	r.router.HandleFunc("/databases", srv.GetDatabases).Methods("GET")
	r.router.HandleFunc("/schemas", srv.GetSchemas).Methods("GET")
	r.router.HandleFunc("/tables", srv.GetTables).Methods("GET")

	// v2: add this route to the router
	// breaking change
	r.router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", srv.ExecuteFromScripts)
	// r.router.HandleFunc("/_QUERIES/{database}/{queriesLocation}/{script}", srv.ExecuteFromScripts)

	// if it is windows it should not register the plugin endpoint
	// we use go plugin system that does not support windows
	// https://github.com/golang/go/issues/19282
	if runtime.GOOS != "windows" {
		r.router.HandleFunc("/_PLUGIN/{file}/{func}", srv.Plugin)
	}

	r.router.HandleFunc("/{database}/{schema}", srv.GetTablesByDatabaseAndSchema).Methods("GET")
	r.router.HandleFunc("/show/{database}/{schema}/{table}", srv.ShowTable).Methods("GET")

	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	r.router.HandleFunc("/_health", srv.WrappedHealthCheck(controllers.DefaultCheckList)).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", srv.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", srv.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", srv.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", srv.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", srv.UpdateTable).Methods("PUT", "PATCH")

	r.router.PathPrefix("/").Handler(
		negroni.New(
			middlewares.ExposureMiddleware(&r.srvCfg.ExposeConf),
			middlewares.AccessControl(srv.GetAdapter().TablePermissions),
			middlewares.AuthMiddleware(
				r.srvCfg.AuthEnabled, r.srvCfg.JWTKey, r.srvCfg.JWTWhiteList),
			middlewares.CacheMiddleware(r.srvCfg, r.cache),
			// plugins middleware
			plugins.MiddlewarePlugin(r.srvCfg.PluginPath, r.srvCfg.PluginMiddlewareList),
			negroni.Wrap(crudRoutes),
		),
	)
	return nil
}
