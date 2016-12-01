package main

import (
	"fmt"
	"net/http"

	"github.com/auth0/go-jwt-middleware"
	"github.com/caarlos0/env"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"

	"github.com/dgrijalva/jwt-go"
	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/controllers"
)

func main() {
	cfg := config.Prest{}
	env.Parse(&cfg)

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(handlerSet))
	if cfg.JWTKey != "" {
		n.Use(jwtMiddleware(cfg.JWTKey))
	}
	r := mux.NewRouter()
	r.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")
	r.HandleFunc("/schemas", controllers.GetSchemas).Methods("GET")
	r.HandleFunc("/tables", controllers.GetTables).Methods("GET")
	r.HandleFunc("/{database}/{schema}", controllers.GetTablesByDatabaseAndSchema).Methods("GET")
	r.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")

	n.UseHandler(r)
	n.Run(fmt.Sprintf(":%v", cfg.HTTPPort))
}

func handlerSet(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Header().Set("Content-Type", "application/json")
	next(w, r)
}

func jwtMiddleware(key string) negroni.Handler {
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(key), nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})
	return negroni.HandlerFunc(jwtMiddleware.HandlerWithNext)
}
