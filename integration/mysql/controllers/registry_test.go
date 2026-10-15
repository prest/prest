package mysql_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// prestd-registry runs testdata/prest_mysql_registry.toml: alias "store" opens
// the shop database and alias "archive" opens other, with the same MySQL user
// (granted on both).

func TestMySQLRegistryAliasServesOwnDatabase(t *testing.T) {
	// Each alias reads tables in its own database.
	base := helpers.RegistryServerURL(t)
	testutils.DoRequest(t, base+"/store/shop/items", nil, http.MethodGet, http.StatusOK, "store reads shop", "ada")
	testutils.DoRequest(t, base+"/archive/other/tags", nil, http.MethodGet, http.StatusOK, "archive reads other")
}

func TestMySQLRegistryAliasCannotReachOtherDatabase(t *testing.T) {
	// The MySQL user can read shop, but the archive alias is pinned to other:
	// CRUD, table listing, and /show on shop through archive are all 404.
	base := helpers.RegistryServerURL(t)
	testutils.DoRequest(t, base+"/archive/shop/items", nil, http.MethodGet, http.StatusNotFound, "archive crud on shop")
	testutils.DoRequest(t, base+"/archive/shop", nil, http.MethodGet, http.StatusNotFound, "archive lists shop tables")
	testutils.DoRequest(t, base+"/show/archive/shop/items", nil, http.MethodGet, http.StatusNotFound, "archive show on shop")
}

func TestMySQLRegistryDatabasesListsAliases(t *testing.T) {
	// In registry mode /databases lists the configured aliases with their
	// physical database name, not every database the MySQL user can see.
	base := helpers.RegistryServerURL(t)
	testutils.DoRequest(t, base+"/databases", nil, http.MethodGet, http.StatusOK, "databases",
		`"datname":"store"`, `"datname":"archive"`, `"datname":"down"`, `"physical_name":"shop"`, `"physical_name":"other"`)
}

func TestMySQLRegistryUnreachableAliasIs503(t *testing.T) {
	// The "down" alias points at a closed port: it answers 503 without leaking
	// the driver error (host or port).
	base := helpers.RegistryServerURL(t)
	resp, err := http.Get(base + "/down/shop/items")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "body: %s", body)
	require.Contains(t, string(body), "database unavailable")
	require.NotContains(t, string(body), "3399")
}

func TestMySQLRegistryAllWritesPinned(t *testing.T) {
	// Every write through archive against shop is 404; shop's seeded row is
	// still there afterwards.
	base := helpers.RegistryServerURL(t)
	cross := base + "/archive/shop/items?name=$eq.nope"
	testutils.DoRequest(t, base+"/archive/shop/items", map[string]any{"name": "x"}, http.MethodPost, http.StatusNotFound, "archive insert on shop")
	testutils.DoRequest(t, cross, map[string]any{"name": "x"}, http.MethodPatch, http.StatusNotFound, "archive patch on shop")
	testutils.DoRequest(t, cross, map[string]any{"name": "x"}, http.MethodPut, http.StatusNotFound, "archive put on shop")
	testutils.DoRequest(t, cross, nil, http.MethodDelete, http.StatusNotFound, "archive delete on shop")
	testutils.DoRequest(t, base+"/batch/archive/shop/items", []map[string]any{{"name": "y"}}, http.MethodPost, http.StatusNotFound, "archive batch on shop")
	testutils.DoRequest(t, base+"/store/shop/items?name=$eq.ada", nil, http.MethodGet, http.StatusOK, "store row intact", "ada")
}

func TestMySQLRegistryMCPSchemaPinned(t *testing.T) {
	// MCP pins each alias to its physical database, like REST.
	base := helpers.RegistryServerURL(t)
	cross := map[string]any{"database": "archive", "schema": "shop", "table": "items"}
	testutils.DoRequest(t, base+"/_mcp", mcpCall(40, "prest.select_table", cross), http.MethodPost, http.StatusBadRequest, "mcp archive select shop", "schema not served")
	testutils.DoRequest(t, base+"/_mcp", mcpCall(41, "prest.list_tables", map[string]any{"database": "archive", "schema": "shop"}), http.MethodPost, http.StatusBadRequest, "mcp archive lists shop", "schema not served")
	testutils.DoRequest(t, base+"/_mcp", mcpCall(42, "prest.list_tables", map[string]any{"database": "archive", "schema": "other"}), http.MethodPost, http.StatusOK, "mcp archive lists other", "tags")
}

func TestMySQLRegistryTablesListServedOnly(t *testing.T) {
	// /tables uses the default adapter and lists only its physical database,
	// never the granted-but-unserved hidden database.
	base := helpers.RegistryServerURL(t)
	status, body := rawRequest(t, http.MethodGet, base+"/tables", nil)
	require.Equal(t, http.StatusOK, status)
	require.NotContains(t, body, "secret")
	require.NotContains(t, body, "hidden")
}
