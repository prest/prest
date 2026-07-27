package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
)

// TestMCPDescribeTable_EnforcesPermissions guards describe_table exposing
// full table schema (column names/types) regardless of per-user permissions.
// Unlike select_table, describeTable used to call describeColumns directly
// with no selectableColumns/TablePermissions/FieldsPermissions check, so it
// leaked schema metadata even for tables the user has no read access to, and
// never filtered columns down to what FieldsPermissions actually grants.
func TestMCPDescribeTable_EnforcesPermissions(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	var testCases = []struct {
		description  string
		table        string
		status       int
		expectedBody []string
	}{
		{
			"describe_table on a table absent from access.tables is denied, not disclosed",
			"test7",
			http.StatusBadRequest,
			[]string{`"error"`, "permission"},
		},
		{
			"describe_table on an allowed table returns only its configured columns",
			"test_readonly_access",
			http.StatusOK,
			[]string{`"name":"id"`, `"name":"name"`, `"count":2`},
		},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		payload := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "prest.describe_table",
				"arguments": map[string]any{
					"database": "prest-test",
					"schema":   "public",
					"table":    tc.table,
				},
			},
		}
		helpers.DoAuthRequest(t, base+"/_mcp", payload, http.MethodPost, token, tc.status, "MCPDescribeTableACL", tc.expectedBody...)
	}
}
