package main

import (
	"fmt"

	"github.com/caarlos0/env"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx"

	"github.com/nuveo/prest/config"
)

var conn *pgx.Conn

func main() {
	cfg := config.Prest{}
	env.Parse(&cfg)

	router := mux.NewRouter()
	n := negroni.Classic()
	n.UseHandler(router)
	n.Run(fmt.Sprintf(":%v", cfg.HTTPPort))
}
