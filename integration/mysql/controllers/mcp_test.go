package mysql_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func mcpCall(id int, tool string, args map[string]any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	}
}

func TestMySQLMCPToolsList(t *testing.T) {
	// tools/list advertises the generic and per-table select tools for shop.items.
	base := helpers.ServerURL(t)
	payload := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}
	testutils.DoRequest(t, base+"/_mcp", payload, http.MethodPost, http.StatusOK, "tools/list", "prest.select_table", "prest.select.shop.shop.items")
}

func TestMySQLMCPSelectTable(t *testing.T) {
	// select_table quotes with backticks and binds filters as ?, so a filtered,
	// ordered, limited read returns the seeded row instead of a syntax error.
	base := helpers.ServerURL(t)
	args := map[string]any{
		"database": "shop", "schema": "shop", "table": "items",
		"filters": map[string]any{"name": "ada"}, "order_by": []string{"-id"}, "limit": 1,
	}
	testutils.DoRequest(t, base+"/_mcp", mcpCall(2, "prest.select_table", args), http.MethodPost, http.StatusOK, "select_table", `"name":"ada"`, `"count":1`)
}

func TestMySQLMCPDescribeTable(t *testing.T) {
	// describe_table reads the MySQL catalog and lists the JSON column.
	base := helpers.ServerURL(t)
	args := map[string]any{"database": "shop", "schema": "shop", "table": "items"}
	testutils.DoRequest(t, base+"/_mcp", mcpCall(3, "prest.describe_table", args), http.MethodPost, http.StatusOK, "describe_table", "meta")
}
