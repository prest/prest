package helpers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	pctx "github.com/prest/prest/v2/context"
)

// Databases returns the database names used by integration tests (see testdata/runtest.sh).
func Databases() []string {
	return []string{"prest-test", "secondary-db"}
}

// TestdataDir returns the absolute path to the repo testdata directory.
func TestdataDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "testdata"
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "testdata"))
}

// PluginLibDir returns the absolute path to the repo lib/ directory for plugin tests.
func PluginLibDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "lib"
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "lib"))
}

// TestConfigPath returns the path to prest.toml for integration tests.
// It respects PREST_CONF when set (e.g. by docker-compose-test.yml).
func TestConfigPath() string {
	if p := os.Getenv("PREST_CONF"); p != "" {
		return p
	}
	return filepath.Join(TestdataDir(), "prest.toml")
}

// TestExposeConfigPath returns the path to prest_expose.toml for integration tests.
func TestExposeConfigPath() string {
	return filepath.Join(TestdataDir(), "prest_expose.toml")
}

// EnsureTestConfigEnv sets PREST_CONF when it is not already set.
func EnsureTestConfigEnv() {
	if os.Getenv("PREST_CONF") == "" {
		os.Setenv("PREST_CONF", TestConfigPath())
	}
}

// LoadTestConfig loads application config and connects to the test database.
func LoadTestConfig(t *testing.T) {
	t.Helper()
	if os.Getenv("PREST_CONF") == "" {
		t.Setenv("PREST_CONF", TestConfigPath())
	}
	config.Load()
	postgres.Load()
}

// VerifyTestDatabases asserts the configured default database matches test expectations.
func VerifyTestDatabases(t *testing.T) {
	t.Helper()
	if config.PrestConf.PGDatabase != "prest-test" {
		t.Fatalf("expected db: 'prest-test', got: %s", config.PrestConf.PGDatabase)
	}
	if config.PrestConf.Adapter.GetDatabase() != "prest-test" {
		t.Fatalf("expected Adapter db: 'prest-test', got: %s", config.PrestConf.Adapter.GetDatabase())
	}
}

// NewIntegrationHandlers returns handlers wired to the test database.
func NewIntegrationHandlers(t *testing.T) *controllers.Handlers {
	t.Helper()
	LoadTestConfig(t)
	return controllers.NewHandlersFromConfig(config.PrestConf)
}

// WithHTTPTimeout sets the request context timeout expected by CRUD handlers.
func WithHTTPTimeout(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pctx.HTTPTimeoutKey, 60))) //nolint:staticcheck
	}
}
