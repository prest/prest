package connection_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/integration/helpers"
)

// backendApplicationName opens a pooled connection through a fresh adapter built
// from cfg and returns the application_name Postgres reports for that backend.
func backendApplicationName(t *testing.T, cfg *config.Prest) string {
	t.Helper()

	pg := postgres.New(cfg)
	if err := postgres.Connect(pg); err != nil {
		t.Fatalf("expected the adapter to connect, but got: %v", err)
	}
	t.Cleanup(func() { postgres.Close(pg) })

	db, err := postgres.DB(pg)
	if err != nil {
		t.Fatalf("expected a pooled connection, but got: %v", err)
	}
	var name string
	err = db.QueryRow(`SELECT application_name FROM pg_stat_activity WHERE pid = pg_backend_pid()`).Scan(&name)
	if err != nil {
		t.Fatalf("expected pg_stat_activity to report the backend, but got: %v", err)
	}
	return name
}

func TestApplicationName(t *testing.T) {
	base := helpers.LoadTestConfig(t)

	// A key=value connection URI that never mentions application_name.
	// Expected: the backend is named after pg.application_name.
	// Why: the name is what an operator filters pg_stat_activity and the server log by.
	t.Run("pooled connection reports pg.application_name", func(t *testing.T) {
		cfg := *base
		cfg.Adapter = nil
		cfg.PGApplicationName = "prestd"
		if got := backendApplicationName(t, &cfg); got != "prestd" {
			t.Fatalf("expected application_name %q, got %q", "prestd", got)
		}
	})

	// The same connection with pg.application_name set to a custom value.
	// Expected: the backend reports the custom value, not the default.
	t.Run("custom pg.application_name is reported", func(t *testing.T) {
		cfg := *base
		cfg.Adapter = nil
		cfg.PGApplicationName = "orders-api"
		if got := backendApplicationName(t, &cfg); got != "orders-api" {
			t.Fatalf("expected application_name %q, got %q", "orders-api", got)
		}
	})

	// A registry URL that already sets application_name=from-url.
	// Expected: the backend keeps from-url even though pg.application_name is "prestd".
	// Why: an operator's explicit name in the URL outranks the configured fallback.
	t.Run("application_name in the URL wins", func(t *testing.T) {
		cfg := *base
		cfg.Adapter = nil
		cfg.PGApplicationName = "prestd"
		dsn := url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(cfg.PGUser, cfg.PGPass),
			Host:     fmt.Sprintf("%s:%d", cfg.PGHost, cfg.PGPort),
			Path:     "/" + cfg.PGDatabase,
			RawQuery: "sslmode=" + cfg.PGSSLMode + "&application_name=from-url",
		}
		cfg.Databases = []config.DatabaseConf{{Alias: "named", URL: dsn.String()}}
		cfg.PGDatabase = "named"
		if got := backendApplicationName(t, &cfg); got != "from-url" {
			t.Fatalf("expected application_name %q, got %q", "from-url", got)
		}
	})

	// pg.application_name is empty, which turns the feature off.
	// Expected: the backend carries no name from prestd (empty, not "prestd").
	t.Run("empty pg.application_name disables the name", func(t *testing.T) {
		cfg := *base
		cfg.Adapter = nil
		cfg.PGApplicationName = ""
		if got := backendApplicationName(t, &cfg); got == "prestd" {
			t.Fatalf("expected no prestd application_name, got %q", got)
		}
	})
}
