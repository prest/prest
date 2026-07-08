package controllers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

func TestMCPDiscovery(t *testing.T) {
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/_mcp", nil, http.MethodGet, http.StatusOK, "MCPDiscovery",
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
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})
	require.NoError(t, err)

	testutils.DoRequest(t, base+"/_mcp", bytes.NewReader(body), http.MethodPost, http.StatusOK, "MCPInitialize",
		`"serverInfo"`,
		`"name":"prest"`,
		`"instructions"`,
	)
}

func TestMCPToolsList(t *testing.T) {
	base := helpers.ServerURL(t)
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})
	require.NoError(t, err)

	testutils.DoRequest(t, base+"/_mcp", bytes.NewReader(body), http.MethodPost, http.StatusOK, "MCPToolsList",
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
		name         string
		payload      map[string]any
		status       int
		expectedBody string
	}{
		{
			name: "ListDatabases",
			payload: map[string]any{
				"jsonrpc": "2.0",
				"id":      3,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "prest.list_databases",
				},
			},
			status:       http.StatusOK,
			expectedBody: `"prest-test"`,
		},
		{
			name: "ListSchemas",
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
			name: "ListTables",
			payload: map[string]any{
				"jsonrpc": "2.0",
				"id":      5,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "prest.list_tables",
				},
			},
			status:       http.StatusOK,
			expectedBody: `"users"`,
		},
		{
			name: "DescribeTable",
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
			name: "SelectTable",
			payload: map[string]any{
				"jsonrpc": "2.0",
				"id":      7,
				"method":  "tools/call",
				"params": map[string]any{
					"name": "prest.select.prest-test.public.test",
				},
			},
			status:       http.StatusOK,
			expectedBody: `"rows"`,
		},
	}

	for _, tc := range toolCalls {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.payload)
			require.NoError(t, err)
			testutils.DoRequest(t, base+"/_mcp", bytes.NewReader(body), http.MethodPost, tc.status, tc.name, tc.expectedBody)
		})
	}
}

func TestMCPUnsupportedTool(t *testing.T) {
	base := helpers.ServerURL(t)
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      8,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "prest.drop_table",
		},
	})
	require.NoError(t, err)

	testutils.DoRequest(t, base+"/_mcp", bytes.NewReader(body), http.MethodPost, http.StatusBadRequest, "MCPUnsupportedTool",
		`unsupported tool`,
	)
}

func TestMCPMultiClusterDiscovery(t *testing.T) {
	base := helpers.MultiClusterServerURL(t)
	testutils.DoRequest(t, base+"/_mcp", nil, http.MethodGet, http.StatusOK, "MCPMultiClusterDiscovery",
		`"name":"prest"`,
		`"tools"`,
	)
}