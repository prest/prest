package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestMCPDiscovery(t *testing.T) {
	base := helpers.ServerURL(t)

	// Discover MCP server metadata and tool catalog via GET.
	// Expected to succeed with HTTP status OK and advertise core list_* tools.
	testutils.DoRequest(
		t, base+"/_mcp",
		nil, http.MethodGet, http.StatusOK, "MCPDiscovery",
		`"name":"prest"`,
		`"protocol":"0.1"`,
		`"tools"`,
		`prest.list_databases`,
		`prest.list_schemas`,
		`prest.list_tables`,
	)
}

func TestMCPInitialize(t *testing.T) {
	base := helpers.ServerURL(t)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}

	// JSON-RPC initialize handshake.
	// Expected to succeed with HTTP status OK and return serverInfo.
	testutils.DoRequest(
		t, base+"/_mcp",
		payload, http.MethodPost, http.StatusOK, "MCPInitialize",
		`"serverInfo"`,
		`"name":"prest"`,
		`"instructions"`,
	)
}

func TestMCPToolsList(t *testing.T) {
	base := helpers.ServerURL(t)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}

	// List MCP tools via JSON-RPC tools/list.
	// Expected to succeed with HTTP status OK and include catalog + select tools.
	testutils.DoRequest(
		t, base+"/_mcp",
		payload, http.MethodPost, http.StatusOK, "MCPToolsList",
		`"tools"`,
		`prest.list_databases`,
		`prest.list_schemas`,
		`prest.list_tables`,
		`prest.describe_table`,
		`prest.select_table`,
	)
}

func TestMCPToolCalls(t *testing.T) {
	base := helpers.ServerURL(t)

	toolCalls := []struct {
		description  string
		payload      map[string]any
		status       int
		expectedBody string
	}{
		{
			description: "Call prest.list_databases; expect OK and prest-test physical_name",
			payload: map[string]any{
				"jsonrpc": "2.0",
				"id":      3,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "prest.list_databases",
				},
			},
			status:       http.StatusOK,
			expectedBody: `"physical_name":"prest-test"`,
		},
		{
			description: "Call prest.list_schemas; expect OK and public schema",
			payload: map[string]any{
				"jsonrpc": "2.0",
				"id":      4,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "prest.list_schemas",
				},
			},
			status:       http.StatusOK,
			expectedBody: `"public"`,
		},
		{
			description: "Call prest.list_tables; expect OK and test table name",
			payload: map[string]any{
				"jsonrpc": "2.0",
				"id":      5,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "prest.list_tables",
				},
			},
			status:       http.StatusOK,
			expectedBody: `"name":"test"`,
		},
		{
			description: "Call prest.describe_table for public.test; expect OK and columns",
			payload: map[string]any{
				"jsonrpc": "2.0",
				"id":      6,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "prest.describe_table",
					"arguments": map[string]any{
						"database": "prest-test",
						"schema":   "public",
						"table":    "test",
					},
				},
			},
			status:       http.StatusOK,
			expectedBody: `"columns"`,
		},
		{
			// Reply carries a single stable seed row ("prest tester") and is
			// never mutated by the integration suite, so this assertion stays
			// reliable even when other packages run concurrently against the
			// shared database (unlike the "test" table, which is emptied by the
			// delete CRUD/router tests).
			description: "Call select on Reply filtered by name; expect OK and prest tester",
			payload: map[string]any{
				"jsonrpc": "2.0",
				"id":      7,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "prest.select.prest-test.public.Reply",
					"arguments": map[string]any{
						"columns":  []string{"id", "name"},
						"filters":  map[string]any{"name": "prest tester"},
						"order_by": []string{"id"},
						"limit":    5,
						"offset":   0,
					},
				},
			},
			status:       http.StatusOK,
			expectedBody: `"prest tester"`,
		},
	}

	for _, tc := range toolCalls {
		t.Run(tc.description, func(t *testing.T) {
			testutils.DoRequest(
				t, base+"/_mcp",
				tc.payload, http.MethodPost, tc.status, tc.description, tc.expectedBody)
		})
	}
}

func TestMCPUnsupportedTool(t *testing.T) {
	base := helpers.ServerURL(t)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "prest.drop_table",
		},
	}

	// Call an unsupported MCP tool name.
	// Expected to fail with HTTP status BadRequest mentioning unsupported tool.
	testutils.DoRequest(
		t, base+"/_mcp",
		payload, http.MethodPost, http.StatusBadRequest, "MCPUnsupportedTool",
		`unsupported tool`,
	)
}
