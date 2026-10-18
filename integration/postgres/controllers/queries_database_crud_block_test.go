package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
)

// TestQueriesDatabase_CRUDCannotPlantStoredSQL guards GHSA-2x9q-r7j9-g85f on
// the database-backed queries server. prest_queries holds the SQL that
// /_QUERIES executes, and the generic CRUD route used to write to it like any
// table, so a caller could plant a template and run it (RCE). AccessControl now
// refuses the reserved table by name. Bodies below are harmless on purpose: if
// the guard regresses, the planted script does nothing.
func TestQueriesDatabase_CRUDCannotPlantStoredSQL(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// Advisory PoC step 1: insert a stored script through the generic CRUD route.
	// Expected 401 because prest_queries is reserved, even for a valid admin JWT.
	helpers.DoAuthRequest(
		t, base+"/prest-test/public/prest_queries",
		map[string]string{
			"location":  "rpt",
			"name":      "d",
			"write_sql": "DO $$ BEGIN PERFORM 1; END $$",
		},
		http.MethodPost, token, http.StatusUnauthorized, "QueriesCRUDPlantDenied")

	// Advisory PoC step 2: execute the script that would have been planted.
	// Expected 401: it was never stored and rpt/d has no ACL grant either,
	// so the executor refuses it before any SQL runs.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/rpt/d",
		nil, http.MethodPost, token, http.StatusUnauthorized, "QueriesCRUDPlantNotExecutable")

	// Overwrite attempt: rewrite read_sql of a permitted script via PATCH.
	// Expected 401 for the same reason; this is the hijack variant of the PoC.
	helpers.DoAuthRequest(
		t, base+"/prest-test/public/prest_queries?location=$eq.fulltable&name=$eq.get_all",
		map[string]string{"read_sql": "SELECT current_user"},
		http.MethodPatch, token, http.StatusUnauthorized, "QueriesCRUDHijackDenied")

	// Same attempt with the DELETE verb and upper-case table name.
	// Expected 401: matching is case-insensitive.
	helpers.DoAuthRequest(
		t, base+"/prest-test/public/PREST_QUERIES?location=$eq.fulltable&name=$eq.get_all",
		nil, http.MethodDelete, token, http.StatusUnauthorized, "QueriesCRUDDeleteDenied")

	// Reading stored SQL and the user table through CRUD is also refused.
	// Expected 401 for both reads.
	helpers.DoAuthRequest(
		t, base+"/prest-test/public/prest_queries",
		nil, http.MethodGet, token, http.StatusUnauthorized, "QueriesCRUDReadQueriesDenied")
	helpers.DoAuthRequest(
		t, base+"/prest-test/public/prest_users",
		nil, http.MethodGet, token, http.StatusUnauthorized, "QueriesCRUDReadUsersDenied")

	// Baseline: the permitted stored script still returns its original result.
	// Expected 200 with the seeded row; the denied PATCH above did not alter it.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/fulltable/get_all?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "QueriesCRUDScriptIntact", "gopher")
}

// TestQueriesDatabase_JoinCannotReachSystemTables guards the read-bypass of
// GHSA-2x9q-r7j9-g85f: joining an ordinary table to prest_users would leak
// credentials without ever naming the reserved table in the URL path.
func TestQueriesDatabase_JoinCannotReachSystemTables(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// _join targeting prest_users from an ordinary table.
	// Expected 400: the join target is reserved for pREST internal use.
	helpers.DoAuthRequest(
		t, base+"/prest-test/public/test"+
			"?_join=inner:prest_users:prest_users.username:$eq:test.name",
		nil, http.MethodGet, token, http.StatusBadRequest, "QueriesJoinUsersDenied",
		"reserved")
}

// TestQueriesDatabase_MCPCannotTouchSystemTables guards the MCP sibling of
// GHSA-2x9q-r7j9-g85f: select_table and describe_table go through
// TablePermissions, which must refuse the reserved tables.
func TestQueriesDatabase_MCPCannotTouchSystemTables(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	var testCases = []struct {
		description string
		tool        string
		table       string
	}{
		{"select_table on prest_queries is denied, not disclosed", "prest.select_table", "prest_queries"},
		{"describe_table on prest_queries is denied, not disclosed", "prest.describe_table", "prest_queries"},
		{"select_table on prest_users is denied, not disclosed", "prest.select_table", "prest_users"},
		{"describe_table on upper-case PREST_USERS is denied", "prest.describe_table", "PREST_USERS"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]any{
				"name": tc.tool,
				"arguments": map[string]any{
					"database": "prest-test",
					"schema":   "public",
					"table":    tc.table,
				},
			},
		}
		// Expected 400 with a JSON-RPC error mentioning permission.
		helpers.DoAuthRequest(
			t, base+"/_mcp", payload, http.MethodPost, token,
			http.StatusBadRequest, "MCPSystemTablesDenied", `"error"`, "permission")
	}
}
