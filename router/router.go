package router

import (
	"net/http"
	"runtime"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/middlewares"
	"github.com/prest/prest/v2/plugins"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"
)

// RegisterRoutes wires HTTP routes onto the given router.
func RegisterRoutes(router *mux.Router, h *controllers.Handlers, crudStack *middlewares.CRUDStack) {
	if config.PrestConf.AuthEnabled {
		router.HandleFunc("/auth", h.Auth.Login).Methods("POST")
	}
	router.HandleFunc("/databases", h.Catalog.ListDatabases).Methods("GET")
	router.HandleFunc("/schemas", h.Catalog.ListSchemas).Methods("GET")
	router.HandleFunc("/tables", h.Catalog.ListTables).Methods("GET")
	router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", h.Script.Execute)
	if runtime.GOOS != "windows" {
		router.HandleFunc("/_PLUGIN/{file}/{func}", plugins.HandlerPlugin)
	}
	router.HandleFunc("/{database}/{schema}", h.Catalog.ListTablesByDatabaseAndSchema).Methods("GET")
	router.HandleFunc("/show/{database}/{schema}/{table}", h.Table.Show).Methods("GET")
	router.HandleFunc("/_health", h.Health.Handler()).Methods("GET")

	router.Handle("/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.Select)).Methods("GET")
	router.Handle("/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.Insert)).Methods("POST")
	router.Handle("/batch/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.BatchInsert)).Methods("POST")
	router.Handle("/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.Delete)).Methods("DELETE")
	router.Handle("/{database}/{schema}/{table}", crudRoute(crudStack, h.CRUD.Update)).Methods("PUT", "PATCH")
}

func crudRoute(stack *middlewares.CRUDStack, handler http.HandlerFunc) http.Handler {
	return negroni.New(append(stack.Handlers(), negroni.Wrap(handler))...)
}

// GetRouter registers all routes using dependencies from config.
func GetRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	h := controllers.NewHandlersFromConfig(config.PrestConf)
	crudStack := middlewares.NewCRUDStack(config.PrestConf)
	RegisterRoutes(router, h, crudStack)
	return router
}

// Routes for pREST
func Routes() *negroni.Negroni {
	n := middlewares.GetApp()
	n.UseHandler(GetRouter())
	return n
}
