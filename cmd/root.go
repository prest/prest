package cmd

import (
	"net/http"
	"os"
	"strconv"

	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/internal/logsafe"

	"log/slog"

	"github.com/spf13/cobra"
)

var (
	prestCfg *config.Prest
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prestd",
	Short: "Serve a RESTful API from any PostgreSQL database",
	Long:  `prestd (PostgreSQL REST), simplify and accelerate development, ⚡ instant, realtime, high-performance on any Postgres application, existing or new`,
	Run: func(cmd *cobra.Command, args []string) {
		prestApp, err := app.New(prestCfg)
		if err != nil {
			slog.Error("initializing app", "err", logsafe.Error(err))
			os.Exit(1)
		}
		startServer(prestCfg, prestApp)
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
func Execute(cfg *config.Prest) {
	prestCfg = cfg

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
	migrateCmd.PersistentFlags().StringVar(&urlConn, "url", driverURL(cfg), "Database driver url")
	migrateCmd.PersistentFlags().StringVar(&path, "path", cfg.MigrationsPath, "Migrations directory")

	if err := RootCmd.Execute(); err != nil {
		slog.Error("executing root command", "err", err)
		os.Exit(1)
	}
}

// startServer starts the server
func startServer(cfg *config.Prest, a *app.App) {
	if err := config.ValidateJWTConfig(cfg); err != nil {
		slog.Error("invalid JWT configuration", "err", err)
		os.Exit(1)
	}

	http.Handle(cfg.ContextPath, a.Handler)

	if !cfg.AccessConf.Restrict {
		slog.Warn("You are running prestd in public mode.")
	}

	if cfg.Debug {
		slog.Warn("You are running prestd in debug mode.")
	}

	address := cfg.HTTPHost + ":" + strconv.Itoa(cfg.HTTPPort)
	slog.Info("listening and serving", slog.String("addr", address), slog.String("context", cfg.ContextPath))

	if cfg.HTTPSMode {
		if err := http.ListenAndServeTLS(address, cfg.HTTPSCert, cfg.HTTPSKey, nil); err != nil {
			slog.Error("HTTPS server failed", "err", err)
			os.Exit(1)
		}
	}
	if err := http.ListenAndServe(address, nil); err != nil {
		slog.Error("HTTP server failed", "err", err)
		os.Exit(1)
	}
}
