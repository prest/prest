package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	nlog "github.com/nuveo/log"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prestd",
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

// startServer starts the server
func startServer() {
	http.Handle(config.PrestConf.ContextPath, controllers.Routes())
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
