package cmd

import (
	"net/http"
	"os"
	"strconv"

	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/router"

	"log/slog"

	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prestd",
	Short: "Serve a RESTful API from any PostgreSQL database",
	Long:  `prestd (PostgreSQL REST), simplify and accelerate development, ⚡ instant, realtime, high-performance on any Postgres application, existing or new`,
	Run: func(cmd *cobra.Command, args []string) {
		if config.PrestConf.Adapter == nil {
			slog.Warn("adapter is not set. Using the default (postgres)")
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
		slog.Error("executing root command", "err", err)
		os.Exit(1)
	}
}

// startServer starts the server
func startServer() {
	http.Handle(config.PrestConf.ContextPath, router.Routes())

	if !config.PrestConf.AccessConf.Restrict {
		slog.Warn("You are running prestd in public mode.")
	}

	if config.PrestConf.Debug {
		slog.Warn("You are running prestd in debug mode.")
	}
	address := config.PrestConf.HTTPHost + ":" + strconv.Itoa(config.PrestConf.HTTPPort)
	slog.Info("listening and serving", slog.String("addr", address), slog.String("context", config.PrestConf.ContextPath))

	if config.PrestConf.HTTPSMode {
		if err := http.ListenAndServeTLS(address, config.PrestConf.HTTPSCert, config.PrestConf.HTTPSKey, nil); err != nil {
			slog.Error("HTTPS server failed", "err", err)
			os.Exit(1)
		}
	}
	if err := http.ListenAndServe(address, nil); err != nil {
		slog.Error("HTTP server failed", "err", err)
		os.Exit(1)
	}
}
