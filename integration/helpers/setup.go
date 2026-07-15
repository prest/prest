package helpers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/middlewares"
	"github.com/prest/prest/v2/plugins"
	"github.com/prest/prest/v2/router"
	"github.com/urfave/negroni/v3"
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
// It respects PREST_CONF when set (e.g. by integration/*/docker-compose.yml).
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

var (
	loadOnce  sync.Once
	loadedCfg *config.Prest // initialized once; callers receive shallow copies
	loadErr   error
)

// SecondaryClusterHost returns the host for the second Postgres cluster in integration tests.
func SecondaryClusterHost() string {
	return os.Getenv("PREST_PG_HOST_B")
}

// LoadMultiClusterConfig loads the multi-cluster test configuration.
// It must not be called from tests that use t.Parallel() because it sets PREST_CONF via t.Setenv.
func LoadMultiClusterConfig(t *testing.T) *config.Prest {
	t.Helper()
	t.Setenv("PREST_CONF", filepath.Join(TestdataDir(), "prest_multicluster.toml"))
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load multi-cluster config: %v", err)
	}
	pg := postgres.New(cfg)
	if err := postgres.Connect(pg); err != nil {
		t.Fatalf("connect multi-cluster config: %v", err)
	}
	t.Cleanup(func() { postgres.Close(pg) })
	cfg.Adapter = pg
	return cfg
}

// LoadTestConfig loads application config and connects to the test database.
// Config and DB connection are initialized once; each call returns a fresh
// shallow copy so per-test field mutations do not leak across tests. The
// shared Adapter pointer is reused (tests mutate config fields, not the adapter).
func LoadTestConfig(t *testing.T) *config.Prest {
	t.Helper()
	if os.Getenv("PREST_CONF") == "" {
		t.Setenv("PREST_CONF", TestConfigPath())
	}
	loadOnce.Do(func() {
		loadedCfg, loadErr = config.Load()
		if loadErr != nil {
			return
		}
		pg := postgres.New(loadedCfg)
		loadErr = postgres.Connect(pg)
		if loadErr != nil {
			return
		}
		loadedCfg.Adapter = pg
	})
	if loadErr != nil {
		t.Fatalf("load test config: %v", loadErr)
	}
	// Integration tests expect catalog and custom routes without default JWT
	// enforcement (matches PREST_DEBUG=true in local docker-compose).
	cfg := *loadedCfg
	cfg.Debug = true
	return &cfg
}

// MiddlewareStack builds the negroni middleware stack for integration tests.
func MiddlewareStack(cfg *config.Prest) *negroni.Negroni {
	testCfg := *cfg
	testCfg.Debug = true
	return middlewares.New(&testCfg)
}

// VerifyTestDatabases asserts the configured default database matches test expectations.
func VerifyTestDatabases(t *testing.T, cfg *config.Prest) {
	t.Helper()
	if cfg.PGDatabase != "prest-test" {
		t.Fatalf("expected db: 'prest-test', got: %s", cfg.PGDatabase)
	}
	if cfg.Adapter.GetDatabase() != "prest-test" {
		t.Fatalf("expected Adapter db: 'prest-test', got: %s", cfg.Adapter.GetDatabase())
	}
}

// NewIntegrationHandlers returns handlers wired to the test database.
func NewIntegrationHandlers(t *testing.T) *controllers.Handlers {
	t.Helper()
	cfg := LoadTestConfig(t)
	return controllers.NewHandlersFromConfig(cfg)
}

// WithHTTPTimeout sets the request context timeout expected by CRUD handlers.
func WithHTTPTimeout(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pctx.HTTPTimeoutKey, 60))) //nolint:staticcheck
	}
}

// IntegrationHandler builds the full negroni middleware stack and router for integration tests.
func IntegrationHandler(t *testing.T, cfg *config.Prest) http.Handler {
	t.Helper()
	h := controllers.NewHandlersFromConfig(cfg)
	plg := plugins.New(cfg)
	crud := middlewares.NewCRUDStack(cfg, plg)
	queryStack := middlewares.NewQueryStack(cfg, middlewares.ScriptPermsFromAdapter(cfg.Adapter))
	var adminStack *middlewares.AdminQueryStack
	if cfg.QueriesConf.RegisterEnabled && cfg.QueriesConf.Storage == config.QueriesStorageDatabase {
		adminStack = middlewares.NewAdminQueryStack(cfg)
	}
	muxRouter := mux.NewRouter().StrictSlash(true)
	router.RegisterRoutes(muxRouter, cfg, h, crud, queryStack, adminStack, plg)
	n := MiddlewareStack(cfg)
	n.UseHandler(muxRouter)
	return n
}
