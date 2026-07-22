package cmd

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/internal/logsafe"

	"log/slog"

	"github.com/spf13/cobra"
)

func withConfig(ctx context.Context, cfg *config.Prest) context.Context {
	return context.WithValue(ctx, pctx.PrestConfigKey, cfg)
}

func configFrom(cmd *cobra.Command) *config.Prest {
	cfg, ok := cmd.Root().Context().Value(pctx.PrestConfigKey).(*config.Prest)
	if !ok || cfg == nil {
		slog.Error("config not initialized")
		os.Exit(1)
	}
	return cfg
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prestd",
	Short: "Serve a RESTful API from any PostgreSQL database",
	Long:  `prestd (PostgreSQL REST), simplify and accelerate development, ⚡ instant, realtime, high-performance on any Postgres application, existing or new`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := configFrom(cmd)
		prestApp, err := app.New(cfg)
		if err != nil {
			slog.Error("initializing app", "err", logsafe.Error(err))
			os.Exit(1)
		}
		startServer(cmd.Context(), cfg, prestApp)
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
func Execute(ctx context.Context, cfg *config.Prest) {
	upCmd.AddCommand(authUpCmd)
	upCmd.AddCommand(queriesUpCmd)
	downCmd.AddCommand(authDownCmd)
	downCmd.AddCommand(queriesDownCmd)
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

	RootCmd.SetContext(withConfig(ctx, cfg))
	if err := RootCmd.Execute(); err != nil {
		slog.Error("executing root command", "err", logsafe.Error(err))
		os.Exit(1)
	}
}

// startServer starts the HTTP server and blocks until it fails or ctx is
// cancelled (SIGINT/SIGTERM), at which point it drains in-flight requests via
// a graceful shutdown so deferred telemetry flushes can run.
func startServer(ctx context.Context, cfg *config.Prest, a *app.App) {
	if !cfg.AccessConf.Restrict {
		slog.Warn("You are running prestd in public mode.")
	}

	if cfg.Debug {
		slog.Warn("You are running prestd in debug mode.")
	}

	// Serve the app handler directly for the default context path. Mounting on a
	// stdlib http.ServeMux sets http.Request.Pattern, which makes otelhttp
	// overwrite span names (e.g. "GET /") at request end, discarding the route
	// template gorilla/mux already tagged. Only wrap for a non-root context path.
	handler := a.Handler
	if cfg.ContextPath != "" && cfg.ContextPath != "/" {
		mux := http.NewServeMux()
		mux.Handle(cfg.ContextPath, a.Handler)
		handler = mux
	}

	address := cfg.HTTPHost + ":" + strconv.Itoa(cfg.HTTPPort)
	srv := &http.Server{Addr: address, Handler: handler}
	slog.Info("listening and serving", slog.String("addr", address), slog.String("context", cfg.ContextPath))

	errCh := make(chan error, 1)
	go func() {
		if cfg.HTTPSMode {
			errCh <- srv.ListenAndServeTLS(cfg.HTTPSCert, cfg.HTTPSKey)
			return
		}
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server failed", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutdown signal received, stopping server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
		}
	}
}
