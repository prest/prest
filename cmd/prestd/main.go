package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prest/prest/v2/cmd"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	// Cancel the root context on SIGINT/SIGTERM so the server can drain and
	// telemetry can flush before exit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown, err := telemetry.Init(ctx, cfg)
	if err != nil {
		slog.Error("initializing telemetry", "err", err)
		os.Exit(1)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(flushCtx); err != nil {
			slog.Error("flushing telemetry", "err", err)
		}
	}()

	cmd.Execute(ctx, cfg)
}
