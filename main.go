package main

import (
	"fmt"

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
	r := mux.NewRouter()
	r.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")

	n.UseHandler(r)
	n.Run(fmt.Sprintf(":%v", cfg.HTTPPort))
}
