package timescaledb_test

import (
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
)

func TestTimescaleQueryRegistryLifecycle(t *testing.T) {
	// The TimescaleDB wrapper must expose the postgres registry and script
	// permissions; without them the queries server would not even start.
	helpers.RunQueryRegistryLifecycle(t, helpers.QueryRegistryTarget{
		BaseURL:   helpers.QueriesServerURL(t),
		Database:  "prest-test",
		AdminUser: "test@postgres.rest",
		AdminPass: "123456",
		JWTKey:    "integration-test-secret-key-32b!!",
	})
}
