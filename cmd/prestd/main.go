package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/prest/prest/v2/cmd"
	"github.com/prest/prest/v2/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("loading config", "err", err)
		os.Exit(1)
	}

	cmd.Execute(context.Background(), cfg)
}
