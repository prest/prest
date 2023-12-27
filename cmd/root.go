package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prest/prest/config"
	"github.com/prest/prest/router"
	"github.com/spf13/cobra"
	slog "github.com/structy/log"
)

var (
	urlConn string
	path    string
	cfg     = config.New()

	ErrPathNotSet = errors.New("Migrations path not set. \nPlease set it using --path flag or in your prest config file")
	ErrURLNotSet  = errors.New("Database URL not set. \nPlease set it using --url flag or configure it on your prest config file")
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prestd",
	Short: "Serve a RESTful API from any PostgreSQL database",
	Long:  `prestd (PostgreSQL REST), simplify and accelerate development, ⚡ instant, realtime, high-performance on any Postgres application, existing or new`,
	Run: func(cmd *cobra.Command, args []string) {
		startServer(config.New())
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
	migrateCmd.PersistentFlags().StringVar(&path, "path", cfg.MigrationsPath, "Migrations directory")

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// startServer starts the server
func startServer(cfg *config.Prest) {
	l := log.New(os.Stdout, "[prestd] ", 0)

	rts, err := router.Routes(cfg)
	if err != nil {
		slog.Fatal(err)
	}

	// pass config and log to router and controllers
	http.Handle(cfg.ContextPath, rts)

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
