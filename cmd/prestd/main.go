package main

import (
	"log/slog"
	"os"

	"github.com/prest/prest/v2/cmd"
	"github.com/prest/prest/v2/config"
)

func main() {
	config.Load()
	// Fail fast when default JWT enforcement is enabled but no verification
	// material was provided — otherwise the middleware would validate bearer
	// tokens against an empty HMAC key. See GHSA-fj7v-859r-2fm4.
	if err := config.ValidateJWTConfig(config.PrestConf); err != nil {
		slog.Error("invalid JWT configuration", "err", err)
		os.Exit(1)
	}
	cmd.Execute()
}
