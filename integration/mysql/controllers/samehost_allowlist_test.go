package mysql_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// The default prestd serves pg.database=shop plus allowed_databases=other, both
// on the same MySQL host. Each database is reachable only as its own {schema}.

func TestMySQLSameHostAllowList_ReadsOwnDatabase(t *testing.T) {
	// Both the primary and the allowed database answer for their own tables.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/other/other/tags", nil, http.MethodGet, http.StatusOK, "other reads own tags")
	testutils.DoRequest(t, base+"/shop/shop/items", nil, http.MethodGet, http.StatusOK, "shop reads own items", "ada")
}

func TestMySQLSameHostAllowList_SchemaPinnedToDatabase(t *testing.T) {
	// {schema} must equal {database}: reaching the sibling database through
	// another entry's pool is 404 for CRUD and /show.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/other/tags", nil, http.MethodGet, http.StatusNotFound, "shop entry on other")
	testutils.DoRequest(t, base+"/other/shop/items", nil, http.MethodGet, http.StatusNotFound, "other entry on shop")
	testutils.DoRequest(t, base+"/show/shop/other/tags", nil, http.MethodGet, http.StatusNotFound, "show shop entry on other")
}

func TestMySQLSameHostAllowList_WritesAreIsolated(t *testing.T) {
	// A row written through /other/other/tags is found there and cannot be
	// reached through the shop entry.
	base := helpers.ServerURL(t)
	name := fmt.Sprintf("samehost-%d", time.Now().UnixNano())
	q := base + "/other/other/tags?name=$eq." + url.QueryEscape(name)
	t.Cleanup(func() {
		testutils.DoRequest(t, q, nil, http.MethodDelete, http.StatusOK, "cleanup tag", "rows_affected")
	})
	testutils.DoRequest(t, base+"/other/other/tags", map[string]any{"name": name}, http.MethodPost, http.StatusCreated, "insert tag", name)
	testutils.DoRequest(t, q, nil, http.MethodGet, http.StatusOK, "read tag in other", name)
	testutils.DoRequest(t, base+"/shop/other/tags?name=$eq."+url.QueryEscape(name), nil, http.MethodGet, http.StatusNotFound, "tag not visible via shop")
	testutils.DoRequest(t, base+"/shop/other/tags", map[string]any{"name": name}, http.MethodPost, http.StatusNotFound, "write via shop entry")
}

func TestMySQLSameHostAllowList_UnlistedDatabaseRejected(t *testing.T) {
	// A database that exists on the host but is not configured is not served.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/mysql/mysql/user", nil, http.MethodGet, http.StatusBadRequest, "unlisted database", "database not registered")
}

func TestMySQLSameHostAllowList_DatabasesListing(t *testing.T) {
	// /databases and /schemas list exactly the served databases.
	base := helpers.ServerURL(t)
	var dbs []map[string]any
	testutils.DoRequestJSON(t, base+"/databases", nil, http.MethodGet, http.StatusOK, "databases", &dbs)
	names := make([]string, 0, len(dbs))
	for _, d := range dbs {
		names = append(names, fmt.Sprint(d["datname"]))
	}
	require.ElementsMatch(t, []string{"shop", "other"}, names)

	var schemas []map[string]any
	testutils.DoRequestJSON(t, base+"/schemas", nil, http.MethodGet, http.StatusOK, "schemas", &schemas)
	require.NotEmpty(t, schemas)
	for _, s := range schemas {
		require.Contains(t, []string{"shop", "other"}, fmt.Sprint(s["schema_name"]), "unexpected schema %v", s)
	}
}

func TestMySQLSameHostAllowList_MCP(t *testing.T) {
	// MCP list_databases reports the served databases only.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/_mcp", mcpCall(20, "prest.list_databases", map[string]any{}), http.MethodPost, http.StatusOK, "list_databases", "shop", "other")
}

// rawRequest sends payload (JSON-encoded when non-nil) and returns the status
// and body, for assertions on what a response must not contain.
func rawRequest(t *testing.T, method, target string, payload any) (int, string) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		require.NoError(t, err)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, target, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(out)
}

func TestMySQLSameHostAllowList_TableListingPinned(t *testing.T) {
	// /{database}/{schema} lists tables only for the database's own schema.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/other", nil, http.MethodGet, http.StatusNotFound, "shop lists other tables", "schema not served")
	testutils.DoRequest(t, base+"/other/other", nil, http.MethodGet, http.StatusOK, "other lists own tables", "tags")
}

func TestMySQLSameHostAllowList_AllWritesPinned(t *testing.T) {
	// Update, delete and batch insert through the shop entry cannot touch
	// other's rows; the seeded row is unchanged afterwards.
	base := helpers.ServerURL(t)
	name := fmt.Sprintf("samehost-w-%d", time.Now().UnixNano())
	own := base + "/other/other/tags?name=$eq." + url.QueryEscape(name)
	cross := base + "/shop/other/tags?name=$eq." + url.QueryEscape(name)
	t.Cleanup(func() {
		testutils.DoRequest(t, own, nil, http.MethodDelete, http.StatusOK, "cleanup tag", "rows_affected")
	})
	testutils.DoRequest(t, base+"/other/other/tags", map[string]any{"name": name}, http.MethodPost, http.StatusCreated, "seed tag", name)

	testutils.DoRequest(t, cross, map[string]any{"name": "x"}, http.MethodPatch, http.StatusNotFound, "patch via shop")
	testutils.DoRequest(t, cross, map[string]any{"name": "x"}, http.MethodPut, http.StatusNotFound, "put via shop")
	testutils.DoRequest(t, cross, nil, http.MethodDelete, http.StatusNotFound, "delete via shop")
	testutils.DoRequest(t, base+"/batch/shop/other/tags", []map[string]any{{"name": "y"}}, http.MethodPost, http.StatusNotFound, "batch via shop")

	testutils.DoRequest(t, own, nil, http.MethodGet, http.StatusOK, "row unchanged", name)
}

func TestMySQLSameHostAllowList_MCPSchemaPinned(t *testing.T) {
	// MCP select, describe and list_tables reject another database as schema.
	base := helpers.ServerURL(t)
	cross := map[string]any{"database": "shop", "schema": "other", "table": "tags"}
	testutils.DoRequest(t, base+"/_mcp", mcpCall(30, "prest.select_table", cross), http.MethodPost, http.StatusBadRequest, "mcp select cross", "schema not served")
	testutils.DoRequest(t, base+"/_mcp", mcpCall(31, "prest.describe_table", cross), http.MethodPost, http.StatusBadRequest, "mcp describe cross", "schema not served")
	testutils.DoRequest(t, base+"/_mcp", mcpCall(32, "prest.list_tables", map[string]any{"database": "shop", "schema": "other"}), http.MethodPost, http.StatusBadRequest, "mcp list_tables cross", "schema not served")
	testutils.DoRequest(t, base+"/_mcp", mcpCall(33, "prest.list_tables", map[string]any{"database": "other", "schema": "other"}), http.MethodPost, http.StatusOK, "mcp list_tables own", "tags")
}

func TestMySQLSameHostAllowList_UnservedDatabaseHidden(t *testing.T) {
	// hidden is granted to the MySQL user but not served: no listing names it
	// and no path reaches it.
	base := helpers.ServerURL(t)
	for _, path := range []string{"/tables", "/databases", "/schemas"} {
		status, body := rawRequest(t, http.MethodGet, base+path, nil)
		require.Equal(t, http.StatusOK, status, path)
		require.NotContains(t, body, "hidden", path)
		require.NotContains(t, body, "secret", path)
	}
	_, tables := rawRequest(t, http.MethodGet, base+"/tables", nil)
	require.Contains(t, tables, "items")
	require.Contains(t, tables, "tags")

	status, body := rawRequest(t, http.MethodPost, base+"/_mcp", mcpCall(34, "prest.list_tables", map[string]any{"database": "shop"}))
	require.Equal(t, http.StatusOK, status)
	require.NotContains(t, body, "secret")

	testutils.DoRequest(t, base+"/hidden/hidden/secret", nil, http.MethodGet, http.StatusBadRequest, "hidden database", "database not registered")
	testutils.DoRequest(t, base+"/shop/hidden/secret", nil, http.MethodGet, http.StatusNotFound, "hidden via shop")
}
