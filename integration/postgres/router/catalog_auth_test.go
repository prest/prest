package router_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

// TestCatalogRoutesRequireToken guards GHSA-fhf5-m388-4w4p: with auth.enabled
// the catalog/metadata routes used to be reachable anonymously.
func TestCatalogRoutesRequireToken(t *testing.T) {
	base := helpers.AuthServerURL(t)

	routes := []struct {
		description string
		path        string
	}{
		{"GET /databases", "/databases"},
		{"GET /schemas", "/schemas"},
		{"GET /tables", "/tables"},
		{"GET /{database}/{schema} lists schema tables", "/prest-test/public"},
		{"GET /show/{db}/{schema}/{table} on a granted table", "/show/prest-test/public/test"},
		{"GET /_PLUGIN/{file}/{func}", "/_PLUGIN/hello/Hello"},
	}
	for _, tc := range routes {
		// No Authorization header on an auth-enabled server.
		// Expected: 401 with the empty-token error, never catalog data.
		testutils.DoRequest(
			t, base+tc.path,
			nil, http.MethodGet, http.StatusUnauthorized,
			tc.description+" without token is Unauthorized",
			"authorization token is empty")
	}

	// Liveness probe must stay public so orchestrators can reach it.
	// Expected: 200 without a token.
	testutils.DoRequest(
		t, base+"/_health",
		nil, http.MethodGet, http.StatusOK, "HealthStaysPublic")
}

// TestCatalogRoutesWithToken guards GHSA-fhf5-m388-4w4p: a valid JWT still
// reaches the catalog routes, while /show goes through table ACL.
func TestCatalogRoutesWithToken(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	routes := []struct {
		description string
		path        string
		status      int
	}{
		{"GET /databases with token returns OK", "/databases", http.StatusOK},
		{"GET /schemas with token returns OK", "/schemas", http.StatusOK},
		{"GET /tables with token returns OK", "/tables", http.StatusOK},
		{"GET /prest-test/public with token returns OK", "/prest-test/public", http.StatusOK},
		{"GET /show on granted table test returns OK", "/show/prest-test/public/test", http.StatusOK},
		// prest_users holds credentials: reserved, denied regardless of config.
		{"GET /show on reserved table prest_users is Unauthorized", "/show/prest-test/public/prest_users", http.StatusUnauthorized},
		// test7 exists but is absent from [access] tables while restrict=true.
		{"GET /show on table not granted in [access] is Unauthorized", "/show/prest-test/public/test7", http.StatusUnauthorized},
	}
	for _, tc := range routes {
		// Authenticated request; the status depends on route and table ACL.
		helpers.DoAuthRequest(
			t, base+tc.path,
			nil, http.MethodGet, token, tc.status, tc.description)
	}
}
