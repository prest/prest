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

	handler := contextPathHandler(cfg.ContextPath, a.Handler)

	address := cfg.HTTPHost + ":" + strconv.Itoa(cfg.HTTPPort)
	srv := &http.Server{
		Addr:    address,
		Handler: handler,
		// Bound how long slow/idle clients may hold a connection. ReadHeaderTimeout
		// mitigates Slowloris; IdleTimeout reaps idle keep-alives. WriteTimeout is
		// intentionally left unset so large/streamed query responses are not
		// truncated; per-request deadlines come from the timeout middleware.
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	slog.Info("listening and serving", slog.String("addr", address), slog.String("context", cfg.ContextPath))

	if err := serveWithShutdown(ctx, cfg, srv); err != nil {
		slog.Error("HTTP server failed", "err", err)
		os.Exit(1)
	}
}

// contextPathHandler mounts h under contextPath. Unlike a stdlib http.ServeMux
// it does not set http.Request.Pattern, so otelhttp preserves the route-template
// span name set by the gorilla/mux router; it also strips the prefix so routes
// resolve. A root context path ("" or "/") returns h unchanged.
func contextPathHandler(contextPath string, h http.Handler) http.Handler {
	if contextPath == "" || contextPath == "/" {
		return h
	}
	return http.StripPrefix(contextPath, h)
}

// serveWithShutdown runs srv until it fails or ctx is cancelled, then drains
// in-flight requests via a graceful shutdown (bounded by shutdownGracePeriod).
// It returns a non-nil error only for an unexpected serve failure, so callers
// can decide how to exit. It is separated from startServer to be testable
// without os.Exit.
func serveWithShutdown(ctx context.Context, cfg *config.Prest, srv *http.Server) error {
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
			return err
		}
		return nil
	case <-ctx.Done():
		slog.Info("shutdown signal received, stopping server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "err", err)
		}
		return nil
	}
}

// shutdownGracePeriod bounds how long a graceful shutdown waits for in-flight
// requests to finish before returning. It is a var so tests can shorten it.
var shutdownGracePeriod = 10 * time.Second
