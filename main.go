package main

import (
	"fmt"
	"net/http"

	"github.com/caarlos0/env"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"

	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/controllers"
)

func main() {
	cfg := config.Prest{}
	env.Parse(&cfg)

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(handlerSet))
	r := mux.NewRouter()
	r.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")
	r.HandleFunc("/tables", controllers.GetTables).Methods("GET")

	n.UseHandler(r)
	n.Run(fmt.Sprintf(":%v", cfg.HTTPPort))
}

func handlerSet(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Header().Set("Content-Type", "application/json")
	next(w, r)
}
