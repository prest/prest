package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleSystemSchemasHidden(t *testing.T) {
	// List schemas without system schemas flag.
	// Expected: _timescaledb_* system schemas should be hidden by default.
	// Only public schema (and standard Postgres system schemas) should be returned.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/schemas",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleSystemSchemasHidden",
		"public", // user schema should be visible
	)
}

func TestTimescaleSystemSchemasVisible(t *testing.T) {
	// List schemas WITH system schemas flag.
	// Expected: _timescaledb_* system schemas should be included.
	// Verifies system schemas can be explicitly requested via _include_system_schemas=true.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/schemas?_include_system_schemas=true",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleSystemSchemasVisible",
		"_timescaledb_cache", // system schema should be visible when flag is set
	)
}
