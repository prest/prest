package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	nlog "github.com/nuveo/log"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/config/router"
	"github.com/prest/prest/controllers"
	"github.com/prest/prest/middlewares"
	"github.com/spf13/cobra"
	"github.com/urfave/negroni"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prest",
	Short: "Serve a RESTful API from any PostgreSQL database",
	Long:  `pREST (PostgreSQL REST), simplify and accelerate development, ⚡ instant, realtime, high-performance on any Postgres application, existing or new`,
	Run: func(cmd *cobra.Command, args []string) {
		if config.PrestConf.Adapter == nil {
			nlog.Warningln("adapter is not set. Using the default (postgres)")
			postgres.Load()
		}
		startServer()
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	upCmd.AddCommand(authUpCmd)
	downCmd.AddCommand(authDownCmd)
	migrateCmd.AddCommand(downCmd)
	migrateCmd.AddCommand(mversionCmd)
	migrateCmd.AddCommand(nextCmd)
	migrateCmd.AddCommand(redoCmd)
	migrateCmd.AddCommand(upCmd)
	migrateCmd.AddCommand(resetCmd)
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(migrateCmd)
	migrateCmd.PersistentFlags().StringVar(&urlConn, "url", driverURL(), "Database driver url")
	migrateCmd.PersistentFlags().StringVar(&path, "path", config.PrestConf.MigrationsPath, "Migrations directory")

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

// MakeHandler reagister all routes
func MakeHandler() http.Handler {
	n := middlewares.GetApp()
	r := router.Get()
	// if auth is enabled
	if config.PrestConf.AuthEnabled {
		r.HandleFunc("/auth", controllers.Auth).Methods("POST")
	}
	r.HandleFunc("/_DATABASES", controllers.GetDatabases).Methods("GET") //TODO: listar as configurações dos bancos, do config
	r.HandleFunc("/_SCHEMAS/{database}", controllers.GetSchemas).Methods("GET")
	r.HandleFunc("/_TABLES/{database}/{schema}", controllers.GetTables).Methods("GET")
	r.HandleFunc("/_QUERIES/{database}/{queriesLocation}/{script}", controllers.ExecuteFromScripts)
	r.HandleFunc("/_SHOW/{database}/{schema}/{table}", controllers.ShowTable).Methods("GET")
	r.HandleFunc("/{database}/{schema}", controllers.GetTablesByDatabaseAndSchema).Methods("GET") // TODO: entender a diff, essa rota e GetTables
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	crudRoutes.HandleFunc("/_BATCH/{database}/{schema}/{table}", controllers.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.InsertInTables).Methods("POST")
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

func startServer() {
	http.Handle(config.PrestConf.ContextPath, MakeHandler())
	l := log.New(os.Stdout, "[prest] ", 0)

	if !config.PrestConf.AccessConf.Restrict {
		nlog.Warningln("You are running pREST in public mode.")
	}

	if config.PrestConf.Debug {
		nlog.DebugMode = config.PrestConf.Debug
		nlog.Warningln("You are running pREST in debug mode.")
	}
	addr := fmt.Sprintf("%s:%d", config.PrestConf.HTTPHost, config.PrestConf.HTTPPort)
	l.Printf("listening on %s and serving on %s", addr, config.PrestConf.ContextPath)
	if config.PrestConf.HTTPSMode {
		l.Fatal(http.ListenAndServeTLS(addr, config.PrestConf.HTTPSCert, config.PrestConf.HTTPSKey, nil))
	}
	l.Fatal(http.ListenAndServe(addr, nil))
}
