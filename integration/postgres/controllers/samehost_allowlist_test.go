package controllers_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

// samehostRows decodes a REST row-list response.
type samehostRows = []map[string]any

func TestSameHostAllowList_ReadsOwnDatabase(t *testing.T) {
	base := helpers.ServerURL(t)

	// only_in_secondary exists solely in secondary-db, which shares a host with prest-test.
	// Expected: 200 through secondary-db and 404 through prest-test.
	testutils.DoRequest(t, base+"/secondary-db/public/only_in_secondary",
		nil, http.MethodGet, http.StatusOK, "SameHostReadSecondary", `"secondary"`)
	testutils.DoRequest(t, base+"/prest-test/public/only_in_secondary",
		nil, http.MethodGet, http.StatusNotFound, "SameHostReadPrimary")
}

func TestSameHostAllowList_WritesAreIsolated(t *testing.T) {
	base := helpers.ServerURL(t)
	name := fmt.Sprintf("iso-%d", time.Now().UnixNano())
	filter := "?name=" + url.QueryEscape("$eq."+name)

	t.Cleanup(func() {
		testutils.DoRequest(t, base+"/secondary-db/public/test"+filter,
			nil, http.MethodDelete, http.StatusOK, "SameHostCleanup")
	})

	// Insert through secondary-db only.
	// Expected: the row is visible there and absent from prest-test.
	testutils.DoRequest(t, base+"/secondary-db/public/test",
		map[string]any{"name": name}, http.MethodPost, http.StatusCreated, "SameHostInsert")

	var secondary, primary samehostRows
	testutils.DoRequestJSON(t, base+"/secondary-db/public/test"+filter,
		nil, http.MethodGet, http.StatusOK, "SameHostSecondaryRows", &secondary)
	assert.Len(t, secondary, 1)
	testutils.DoRequestJSON(t, base+"/prest-test/public/test"+filter,
		nil, http.MethodGet, http.StatusOK, "SameHostPrimaryRows", &primary)
	assert.Empty(t, primary)
}

func TestSameHostAllowList_TablesListedPerDatabase(t *testing.T) {
	base := helpers.ServerURL(t)

	// Table listing is scoped to the requested database.
	// Expected: only_in_secondary appears under secondary-db and not under prest-test.
	secondary := samehostTableNames(t, base+"/secondary-db/public")
	primary := samehostTableNames(t, base+"/prest-test/public")
	assert.Contains(t, secondary, "only_in_secondary")
	assert.NotContains(t, primary, "only_in_secondary")
}

func samehostTableNames(t *testing.T, u string) []string {
	t.Helper()
	var rows samehostRows
	testutils.DoRequestJSON(t, u, nil, http.MethodGet, http.StatusOK, "SameHostTables", &rows)
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		if n, ok := r["table_name"].(string); ok {
			names = append(names, n)
		} else if n, ok := r["name"].(string); ok {
			names = append(names, n)
		}
	}
	return names
}

func TestSameHostAllowList_UnlistedDatabaseRejected(t *testing.T) {
	base := helpers.ServerURL(t)

	// The postgres database exists on the host but is not in pg.allowed_databases.
	// Expected: 400 with "database not registered".
	testutils.DoRequest(t, base+"/postgres/public/pg_tables",
		nil, http.MethodGet, http.StatusBadRequest, "SameHostUnlisted", "database not registered")
}

func TestSameHostAllowList_DatabasesListing(t *testing.T) {
	base := helpers.ServerURL(t)

	// /databases lists only the served databases, not every database on the host.
	// Expected: exactly prest-test and secondary-db.
	var rows samehostRows
	testutils.DoRequestJSON(t, base+"/databases", nil, http.MethodGet, http.StatusOK, "SameHostDatabases", &rows)
	got := make([]string, 0, len(rows))
	for _, r := range rows {
		if n, ok := r["datname"].(string); ok {
			got = append(got, n)
		}
	}
	assert.ElementsMatch(t, helpers.Databases(), got)
}

func TestSameHostAllowList_MCP(t *testing.T) {
	base := helpers.ServerURL(t)

	// MCP list_databases reports both served databases.
	// Expected: 200 with prest-test and secondary-db.
	testutils.DoRequest(t, base+"/_mcp", samehostMCPCall(1, "prest.list_databases", nil),
		http.MethodPost, http.StatusOK, "SameHostMCPList",
		`"physical_name":"prest-test"`, `"physical_name":"secondary-db"`)

	// select_table on a secondary-db-only table.
	// Expected: 200 with the marker row.
	args := map[string]any{"database": "secondary-db", "schema": "public", "table": "only_in_secondary"}
	testutils.DoRequest(t, base+"/_mcp", samehostMCPCall(2, "prest.select_table", args),
		http.MethodPost, http.StatusOK, "SameHostMCPSelect", "secondary")
}

func samehostMCPCall(id int, tool string, args map[string]any) map[string]any {
	params := map[string]any{"name": tool}
	if args != nil {
		params["arguments"] = args
	}
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": params}
}
