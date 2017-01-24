package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/controllers"
	"github.com/spf13/cobra"
	"github.com/urfave/negroni"
)

var cfgFile string
var prestConfig config.Prest

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prest",
	Short: "Serve a RESTful API from any PostgreSQL database",
	Long:  `Serve a RESTful API from any PostgreSQL database, start HTTP server`,
	Run: func(cmd *cobra.Command, args []string) {
		app()
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	config.InitConf()
}

func handlerSet(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Header().Set("Content-Type", "application/json")
	next(w, r)
}

// AccessControl is a middleware to handle permissions on tables in pREST
func AccessControl(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
	mapPath := getVars(rq.URL.Path)
	if mapPath == nil {
		next(rw, rq)
		return
	}

	permission := permissionByMethod(rq.Method)
	if permission == "" {
		next(rw, rq)
		return
	}

	if postgres.TablePermissions(mapPath["table"], permission) {
		next(rw, rq)
		return
	}

	err := fmt.Errorf("required authorization to table %s", mapPath["table"])
	http.Error(rw, err.Error(), http.StatusUnauthorized)
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

func app() {
	cfg := config.Prest{}
	config.Parse(&cfg)

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(handlerSet))
	n.Use(negroni.HandlerFunc(AccessControl))
	if cfg.JWTKey != "" {
		n.Use(jwtMiddleware(cfg.JWTKey))
	}
	r := mux.NewRouter()
	r.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")
	r.HandleFunc("/schemas", controllers.GetSchemas).Methods("GET")
	r.HandleFunc("/tables", controllers.GetTables).Methods("GET")
	r.HandleFunc("/{database}/{schema}", controllers.GetTablesByDatabaseAndSchema).Methods("GET")
	r.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")
	r.HandleFunc("/{database}/{schema}/{table}", controllers.InsertInTables).Methods("POST")
	r.HandleFunc("/{database}/{schema}/{table}", controllers.DeleteFromTable).Methods("DELETE")
	r.HandleFunc("/{database}/{schema}/{table}", controllers.UpdateTable).Methods("PUT", "PATCH")
	r.HandleFunc("/_VIEW/{database}/{schema}/{view}", controllers.SelectFromViews).Methods("GET")

	n.UseHandler(r)
	n.Run(fmt.Sprintf(":%v", cfg.HTTPPort))
}
