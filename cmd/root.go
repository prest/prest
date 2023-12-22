package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/router"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prestd",
	Short: "Serve a RESTful API from any PostgreSQL database",
	Long:  `prestd (PostgreSQL REST), simplify and accelerate development, ⚡ instant, realtime, high-performance on any Postgres application, existing or new`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.New()
		if cfg.Adapter == nil {
			slog.Warningln("adapter is not set. Using the default (postgres)")
			cfg.Adapter = postgres.NewAdapter(cfg)
			// todo: load db name from config
		}
		startServer(cfg)
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
		os.Exit(1)
	}
}

// startServer starts the server
func startServer(cfg *config.Prest) {
	// pass config and log to router and controllers
	http.Handle(cfg.ContextPath, router.Routes(cfg))
	l := log.New(os.Stdout, "[prestd] ", 0)

	if !cfg.AccessConf.Restrict {
		slog.Warningln("You are running prestd in public mode.")
	}

	if cfg.Debug {
		slog.DebugMode = cfg.Debug
		slog.Warningln("You are running prestd in debug mode.")
	}
	addr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.HTTPPort)
	l.Printf("listening on %s and serving on %s", addr, cfg.ContextPath)
	if cfg.HTTPSMode {
		l.Fatal(http.ListenAndServeTLS(addr, cfg.HTTPSCert, cfg.HTTPSKey, nil))
	}
	l.Fatal(http.ListenAndServe(addr, nil))
}
