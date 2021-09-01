package controllers

import (
	"github.com/gorilla/mux"
	"github.com/prest/prest/config"
	"github.com/prest/prest/config/router"
	"github.com/prest/prest/middlewares"
	"github.com/urfave/negroni"
)

// MakeHandler reagister all routes
func Routes() *negroni.Negroni {
	n := middlewares.GetApp()
	r := router.Get()
	// if auth is enabled
	if config.PrestConf.AuthEnabled {
		r.HandleFunc("/auth", Auth).Methods("POST")
	}
	r.HandleFunc("/databases", GetDatabases).Methods("GET")
	r.HandleFunc("/schemas", GetSchemas).Methods("GET")
	r.HandleFunc("/tables", GetTables).Methods("GET")
	r.HandleFunc("/_QUERIES/{queriesLocation}/{script}", ExecuteFromScripts)
	r.HandleFunc("/{database}/{schema}", GetTablesByDatabaseAndSchema).Methods("GET")
	r.HandleFunc("/show/{database}/{schema}/{table}", ShowTable).Methods("GET")
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", UpdateTable).Methods("PUT", "PATCH")
	r.PathPrefix("/").Handler(negroni.New(
		middlewares.AccessControl(),
		middlewares.AuthMiddleware(),
		negroni.Wrap(crudRoutes),
	))
	n.UseHandler(r)
	return n
}
