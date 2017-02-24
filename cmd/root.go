package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/controllers"
	"github.com/nuveo/prest/middlewares"
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

func app() {
	cfg := config.Prest{}

	err := config.Parse(&cfg)
	if err != nil {
		log.Fatalf("Error parsing conf: %s", err)
	}

	n := negroni.Classic()
	n.Use(negroni.HandlerFunc(middlewares.HandlerSet))
	if cfg.JWTKey != "" {
		n.Use(middlewares.JwtMiddleware(cfg.JWTKey))
	}

	r := config.GetRouter()

	r.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")
	r.HandleFunc("/schemas", controllers.GetSchemas).Methods("GET")
	r.HandleFunc("/tables", controllers.GetTables).Methods("GET")
	r.HandleFunc("/_QUERIES/{queriesLocation}/{script}", controllers.ExecuteFromScripts)
	r.HandleFunc("/{database}/{schema}", controllers.GetTablesByDatabaseAndSchema).Methods("GET")

	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)

	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.UpdateTable).Methods("PUT", "PATCH")

	r.PathPrefix("/").Handler(negroni.New(
		negroni.HandlerFunc(middlewares.AccessControl),
		negroni.Wrap(crudRoutes),
	))

	if cfg.CORSAllowOrigin != nil {
		c := cors.New(cors.Options{
			AllowedOrigins: cfg.CORSAllowOrigin,
		})
		n.Use(c)
	}

	n.UseHandler(r)
	n.Run(fmt.Sprintf(":%v", cfg.HTTPPort))
}
