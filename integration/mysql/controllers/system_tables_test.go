package mysql_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

// TestMySQLSystemTables_DeniedThroughGenericRoutes guards GHSA-2x9q-r7j9-g85f
// on MySQL: prest_queries (stored SQL executed by /_QUERIES) and prest_users
// (credentials) were writable and readable through the generic table routes
// under the advisory default access.restrict=false. TableAllowed now denies
// them by name in every configuration, so AccessControl answers 401.
func TestMySQLSystemTables_DeniedThroughGenericRoutes(t *testing.T) {
	base := helpers.ServerURL(t)
	body := map[string]string{"name": "ghsa-2x9q"}
	copyHeader := map[string]string{"Prest-Batch-Method": "copy"}

	var testCases = []struct {
		description string
		url         string
		method      string
		body        interface{}
		headers     map[string]string
	}{
		{"GET on prest_queries is denied", "/shop/shop/prest_queries", http.MethodGet, nil, nil},
		{"POST on prest_queries is denied", "/shop/shop/prest_queries", http.MethodPost, body, nil},
		{"PUT on prest_queries is denied", "/shop/shop/prest_queries?name=$eq.x", http.MethodPut, body, nil},
		{"PATCH on prest_queries is denied", "/shop/shop/prest_queries?name=$eq.x", http.MethodPatch, body, nil},
		{"DELETE on prest_queries is denied", "/shop/shop/prest_queries?name=$eq.x", http.MethodDelete, nil, nil},
		{"GET on prest_users is denied", "/shop/shop/prest_users", http.MethodGet, nil, nil},
		{"POST on prest_users is denied", "/shop/shop/prest_users", http.MethodPost, body, nil},
		{"upper-case PREST_QUERIES path is denied (case-insensitive match)", "/shop/shop/PREST_QUERIES", http.MethodGet, nil, nil},
		{"batch insert into prest_queries is denied", "/batch/shop/shop/prest_queries", http.MethodPost, []map[string]string{body}, nil},
		{"batch COPY insert into prest_queries is denied", "/batch/shop/shop/prest_queries", http.MethodPost, []map[string]string{body}, copyHeader},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Expected 401: rejected by AccessControl before any SQL runs.
			testutils.DoRequestWithHeaders(
				t, base+tc.url, tc.body, tc.method,
				http.StatusUnauthorized, "MySQLSystemTablesDenied", tc.headers)
		})
	}
}

// TestMySQLSystemTables_DeniedAsJoinTarget guards the read-bypass sibling of
// GHSA-2x9q-r7j9-g85f: an ordinary table could join to a reserved table, which
// AccessControl never sees because only the path table is checked.
// MySQL FieldsPermissions now rejects it and Select answers 400.
func TestMySQLSystemTables_DeniedAsJoinTarget(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		status      int
	}{
		{"join to prest_queries is rejected", "/shop/shop/items?_join=inner:prest_queries:prest_queries.name:$eq:items.name", http.StatusBadRequest},
		{"join to prest_users is rejected", "/shop/shop/items?_join=inner:prest_users:prest_users.username:$eq:items.name", http.StatusBadRequest},
		{"join to upper-case PREST_USERS is rejected", "/shop/shop/items?_join=left:PREST_USERS:PREST_USERS.username:$eq:items.name", http.StatusBadRequest},
		{"baseline: plain select on an ordinary table still works", "/shop/shop/items", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			testutils.DoRequest(
				t, base+tc.url, nil, http.MethodGet, tc.status, "MySQLSystemTablesJoin")
		})
	}
}

// TestMySQLSystemTables_QueriesServerCannotPlantStoredSQL replays the
// GHSA-2x9q-r7j9-g85f PoC against the MySQL queries server: with a valid admin
// JWT, CRUD into prest_queries must still be refused, and the planted script
// must not be executable. Bodies are harmless in case the guard regresses.
func TestMySQLSystemTables_QueriesServerCannotPlantStoredSQL(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	// Insert a stored script through the generic CRUD route.
	// Expected 401 because prest_queries is reserved.
	helpers.DoAuthRequest(
		t, base+"/shop/shop/prest_queries",
		map[string]string{"location": "rpt", "name": "d", "write_sql": "SELECT 1"},
		http.MethodPost, token, http.StatusUnauthorized, "MySQLQueriesCRUDPlantDenied")

	// Execute the script that would have been planted.
	// Expected 401: never stored and not granted, refused before any SQL runs.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/rpt/d",
		nil, http.MethodPost, token, http.StatusUnauthorized, "MySQLQueriesCRUDPlantNotExecutable")

	// Hijack a permitted script by rewriting its read_sql.
	// Expected 401 for the same reason.
	helpers.DoAuthRequest(
		t, base+"/shop/shop/prest_queries?location=$eq.fulltable&name=$eq.get_bound",
		map[string]string{"read_sql": "SELECT 1"},
		http.MethodPatch, token, http.StatusUnauthorized, "MySQLQueriesCRUDHijackDenied")

	// Join an ordinary table to prest_users on the queries server.
	// Expected 400: reserved join target.
	helpers.DoAuthRequest(
		t, base+"/shop/shop/items?_join=inner:prest_users:prest_users.username:$eq:items.name",
		nil, http.MethodGet, token, http.StatusBadRequest, "MySQLQueriesJoinDenied")

	// Baseline: the permitted imported script still runs unchanged.
	// Expected 200 with the seeded value.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/fulltable/get_bound?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "MySQLQueriesScriptIntact", "da silva")
}

// TestMySQLSystemTables_MCPDenied guards the MCP sibling of GHSA-2x9q-r7j9-g85f
// on MySQL: select_table and describe_table must refuse the reserved tables.
func TestMySQLSystemTables_MCPDenied(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		tool        string
		table       string
	}{
		{"select_table on prest_queries is denied", "prest.select_table", "prest_queries"},
		{"describe_table on prest_queries is denied", "prest.describe_table", "prest_queries"},
		{"describe_table on prest_users is denied", "prest.describe_table", "prest_users"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			args := map[string]any{"database": "shop", "schema": "shop", "table": tc.table}
			// Expected 400 with a JSON-RPC error mentioning permission.
			testutils.DoRequest(
				t, base+"/_mcp", mcpCall(9, tc.tool, args), http.MethodPost,
				http.StatusBadRequest, "MySQLMCPSystemTablesDenied", `"error"`, "permission")
		})
	}
}
