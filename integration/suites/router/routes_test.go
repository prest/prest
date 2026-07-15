package router_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestRoutes(t *testing.T) {
	base := helpers.ServerURL(t)

	// Smoke-check that the default router serves health.
	// Expected to succeed with HTTP status OK.
	testutils.DoRequest(
		t, base+"/_health",
		nil, "GET", http.StatusOK, "RoutesHealth")
}

func TestMCPRoute(t *testing.T) {
	base := helpers.ServerURL(t)

	// Discover MCP over GET on /_mcp.
	// Expected to succeed with HTTP status OK.
	testutils.DoRequest(
		t, base+"/_mcp",
		nil, "GET", http.StatusOK, "MCPDiscovery")

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}

	// Initialize MCP over POST on /_mcp.
	// Expected to succeed with HTTP status OK.
	testutils.DoRequest(
		t, base+"/_mcp",
		payload, "POST", http.StatusOK, "MCPInitialize")
}

func TestDefaultRouters(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"GET /databases lists databases", "/databases", "GET", http.StatusOK},
		{"GET /_mcp returns MCP discovery", "/_mcp", "GET", http.StatusOK},
		{"GET /schemas lists schemas", "/schemas", "GET", http.StatusOK},
		{"GET missing custom query returns BadRequest", "/_QUERIES/missing/script", "GET", http.StatusBadRequest},
		// Registry is not registered on the default server; path falls through to catalog.
		{"GET /_QUERIES/registry on default server returns BadRequest", "/_QUERIES/registry", "GET", http.StatusBadRequest},
		{"GET /prest-test/public lists tables", "/prest-test/public", "GET", http.StatusOK},
		{"GET invalid show path returns BadRequest", "/show/0prest-test/public/test", "GET", http.StatusBadRequest},
		{"GET table rows returns OK", "/prest-test/public/test", "GET", http.StatusOK},
		{"POST table without body returns BadRequest", "/prest-test/public/test", "POST", http.StatusBadRequest},
		{"POST batch without body returns BadRequest", "/batch/prest-test/public/test", "POST", http.StatusBadRequest},
		{"DELETE table rows returns OK", "/prest-test/public/test", "DELETE", http.StatusOK},
		{"PUT table without body returns BadRequest", "/prest-test/public/test", "PUT", http.StatusBadRequest},
		{"PATCH table without body returns BadRequest", "/prest-test/public/test", "PATCH", http.StatusBadRequest},
		{"GET /auth on default server returns NotFound", "/auth", "GET", http.StatusNotFound},
		{"GET / returns NotFound", "/", "GET", http.StatusNotFound},
	}
	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, tc.description)
	}
}
