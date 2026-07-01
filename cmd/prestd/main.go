package main

import (
	"log/slog"
	"os"

	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/cmd"
	"github.com/prest/prest/v2/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	prestApp, err := app.New(cfg)
	if err != nil {
		slog.Error("initializing app", "err", err)
		os.Exit(1)
	}

	cmd.Execute(cfg, prestApp)
}
