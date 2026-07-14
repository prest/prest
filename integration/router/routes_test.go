package router_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestRoutes(t *testing.T) {
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/_health", nil, "GET", http.StatusOK, "Routes")
}

func TestMCPRoute(t *testing.T) {
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/_mcp", nil, "GET", http.StatusOK, "MCPDiscovery")

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	}
	testutils.DoRequest(t, base+"/_mcp", payload, "POST", http.StatusOK, "MCPInitialize")
}

func TestDefaultRouters(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		url    string
		method string
		status int
	}{
		{"/databases", "GET", http.StatusOK},
		{"/_mcp", "GET", http.StatusOK},
		{"/schemas", "GET", http.StatusOK},
		{"/_QUERIES/missing/script", "GET", http.StatusBadRequest},
		// Registry is not registered on the default server; path falls through to catalog.
		{"/_QUERIES/registry", "GET", http.StatusBadRequest},
		{"/prest-test/public", "GET", http.StatusOK},
		{"/show/0prest-test/public/test", "GET", http.StatusBadRequest},
		{"/prest-test/public/test", "GET", http.StatusOK},
		{"/prest-test/public/test", "POST", http.StatusBadRequest},
		{"/batch/prest-test/public/test", "POST", http.StatusBadRequest},
		{"/prest-test/public/test", "DELETE", http.StatusOK},
		{"/prest-test/public/test", "PUT", http.StatusBadRequest},
		{"/prest-test/public/test", "PATCH", http.StatusBadRequest},
		{"/auth", "GET", http.StatusNotFound},
		{"/", "GET", http.StatusNotFound},
	}
	for _, tc := range testCases {
		t.Log(tc.method, "\t", tc.url)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, tc.url)
	}
}

func TestAuthRouterActive(t *testing.T) {
	base := helpers.AuthServerURL(t)
	testutils.DoRequest(t, base+"/auth", nil, "GET", http.StatusMethodNotAllowed, "AuthEnable")
	testutils.DoRequest(t, base+"/auth", nil, "POST", http.StatusUnauthorized, "AuthEnable")
	testutils.DoRequest(t, base+"/_mcp", nil, "GET", http.StatusUnauthorized, "MCPAuthRequired")
}

func TestQueriesRegistryRoutes(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", nil, http.MethodGet, token, http.StatusOK, "QueriesRegistryRoutes", "get_all")
}

func TestMultiClusterRoutes(t *testing.T) {
	base := helpers.MultiClusterServerURL(t)

	testutils.DoRequest(t, base+"/prest-test/public/test", nil, "GET", http.StatusOK, "MultiClusterPrimary")
	testutils.DoRequest(t, base+"/secondary-db/public/test", nil, "GET", http.StatusOK, "MultiClusterSecondary")
	testutils.DoRequest(t, base+"/not-registered/public/test", nil, "GET", http.StatusBadRequest, "MultiClusterUnknown")
	testutils.DoRequest(t, base+"/_ready", nil, "GET", http.StatusOK, "MultiClusterReady")
	testutils.DoRequest(t, base+"/databases", nil, "GET", http.StatusOK, "MultiClusterDatabases")
}
