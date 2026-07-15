package router_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestAuthRouterActive(t *testing.T) {
	base := helpers.AuthServerURL(t)

	// GET /auth when auth is enabled should reject wrong method.
	// Expected to fail with HTTP status MethodNotAllowed.
	testutils.DoRequest(
		t, base+"/auth",
		nil, "GET", http.StatusMethodNotAllowed, "AuthEnableGET")

	// POST /auth without credentials when auth is enabled.
	// Expected to fail with HTTP status Unauthorized.
	testutils.DoRequest(
		t, base+"/auth",
		nil, "POST", http.StatusUnauthorized, "AuthEnablePOST")

	// MCP discovery requires a token on the auth-enabled server.
	// Expected to fail with HTTP status Unauthorized.
	testutils.DoRequest(
		t, base+"/_mcp",
		nil, "GET", http.StatusUnauthorized, "MCPAuthRequired")
}

func TestQueriesRegistryRoutes(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	// List query registry as authenticated admin.
	// Expected to succeed with HTTP status OK and include get_all.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry",
		nil, http.MethodGet, token, http.StatusOK, "QueriesRegistryRoutes", "get_all")
}

func TestMultiClusterRoutes(t *testing.T) {
	base := helpers.MultiClusterServerURL(t)

	// Read a table on the primary registered database.
	// Expected to succeed with HTTP status OK.
	testutils.DoRequest(
		t, base+"/prest-test/public/test",
		nil, "GET", http.StatusOK, "MultiClusterPrimary")

	// Read a table on the secondary registered database.
	// Expected to succeed with HTTP status OK.
	testutils.DoRequest(
		t, base+"/secondary-db/public/test",
		nil, "GET", http.StatusOK, "MultiClusterSecondary")

	// Request an unknown database alias.
	// Expected to fail with HTTP status BadRequest.
	testutils.DoRequest(
		t, base+"/not-registered/public/test",
		nil, "GET", http.StatusBadRequest, "MultiClusterUnknown")

	// Readiness on the multicluster prestd.
	// Expected to succeed with HTTP status OK.
	testutils.DoRequest(
		t, base+"/_ready",
		nil, "GET", http.StatusOK, "MultiClusterReady")

	// List databases known to the multicluster registry.
	// Expected to succeed with HTTP status OK.
	testutils.DoRequest(
		t, base+"/databases",
		nil, "GET", http.StatusOK, "MultiClusterDatabases")
}
