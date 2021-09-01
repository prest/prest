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

// GetRouter for pREST
func GetRouter() *mux.Router {
	if router == nil {
		initRouter()
	}
	return router
}

// Routes reagister all routes
func Routes() *negroni.Negroni {
	n := middlewares.GetApp()
	r := GetRouter()
	// if auth is enabled
	if config.PrestConf.AuthEnabled {
		r.HandleFunc("/auth", controllers.Auth).Methods("POST")
	}
	r.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")
	r.HandleFunc("/schemas", controllers.GetSchemas).Methods("GET")
	r.HandleFunc("/tables", controllers.GetTables).Methods("GET")
	r.HandleFunc("/_QUERIES/{queriesLocation}/{script}", controllers.ExecuteFromScripts)
	r.HandleFunc("/{database}/{schema}", controllers.GetTablesByDatabaseAndSchema).Methods("GET")
	r.HandleFunc("/show/{database}/{schema}/{table}", controllers.ShowTable).Methods("GET")
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", controllers.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.UpdateTable).Methods("PUT", "PATCH")
	r.PathPrefix("/").Handler(negroni.New(
		middlewares.AccessControl(),
		middlewares.AuthMiddleware(),
		negroni.Wrap(crudRoutes),
	))
	n.UseHandler(r)
	return n
}
