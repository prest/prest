package router

import (
	"github.com/gorilla/mux"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
	"github.com/prest/prest/middlewares"
	"github.com/urfave/negroni"
)

var router *mux.Router

func initRouter() {
	router = mux.NewRouter().StrictSlash(true)
}

// GetRouter reagister all routes
func GetRouter() *mux.Router {
	if router == nil {
		initRouter()
	}

	// if auth is enabled
	if config.PrestConf.AuthEnabled {
		router.HandleFunc("/auth", controllers.Auth).Methods("POST")
	}
	router.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")
	router.HandleFunc("/schemas", controllers.GetSchemas).Methods("GET")
	router.HandleFunc("/tables", controllers.GetTables).Methods("GET")
	router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", controllers.ExecuteFromScripts)
	router.HandleFunc("/{database}/{schema}", controllers.GetTablesByDatabaseAndSchema).Methods("GET")
	router.HandleFunc("/show/{database}/{schema}/{table}", controllers.ShowTable).Methods("GET")
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", controllers.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.UpdateTable).Methods("PUT", "PATCH")
	router.PathPrefix("/").Handler(negroni.New(
		middlewares.AccessControl(),
		middlewares.AuthMiddleware(),
		middlewares.CacheMiddleware(),
		negroni.Wrap(crudRoutes),
	))

	return router
}

// Routes for pREST
func Routes() *negroni.Negroni {
	n := middlewares.GetApp()
	n.UseHandler(GetRouter())
	return n
}
