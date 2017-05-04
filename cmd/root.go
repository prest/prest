package cmd

import (
	"fmt"
	"os"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	// postgres driver for migrate
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/nuveo/prest/config"
	cfgMiddleware "github.com/nuveo/prest/config/middlewares"
	"github.com/nuveo/prest/config/router"
	"github.com/nuveo/prest/controllers"
	"github.com/nuveo/prest/middlewares"
	"github.com/spf13/cobra"
	"github.com/urfave/negroni"
)

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

func app() {
	n := cfgMiddleware.GetApp()
	r := router.Get()

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
		middlewares.AccessControl(),
		negroni.Wrap(crudRoutes),
	))

	if config.PrestConf.CORSAllowOrigin != nil {
		c := cors.New(cors.Options{
			AllowedOrigins: config.PrestConf.CORSAllowOrigin,
		})
		n.Use(c)
	}

	n.UseHandler(r)
	n.Run(fmt.Sprintf(":%v", config.PrestConf.HTTPPort))
}
