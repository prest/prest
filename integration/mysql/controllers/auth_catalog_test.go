package mysql_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// mysqlCatalogRoutes are the catalog/metadata routes covered by
// GHSA-fhf5-m388-4w4p.
var mysqlCatalogRoutes = []struct {
	description string
	path        string
}{
	{"GET /databases", "/databases"},
	{"GET /schemas", "/schemas"},
	{"GET /tables", "/tables"},
	{"GET /shop/shop lists schema tables", "/shop/shop"},
	{"GET /show on table items", "/show/shop/shop/items"},
}

// TestMySQLCatalogRoutesRequireToken guards GHSA-fhf5-m388-4w4p on MySQL.
func TestMySQLCatalogRoutesRequireToken(t *testing.T) {
	base := helpers.AuthServerURL(t)

	for _, tc := range mysqlCatalogRoutes {
		// No Authorization header on an auth-enabled server.
		// Expected: 401 with the empty-token error, never catalog data.
		testutils.DoRequest(
			t, base+tc.path,
			nil, http.MethodGet, http.StatusUnauthorized,
			tc.description+" without token is Unauthorized",
			"authorization token is empty")
	}

	// Liveness probe must stay public.
	// Expected: 200 without a token.
	testutils.DoRequest(
		t, base+"/_health",
		nil, http.MethodGet, http.StatusOK, "HealthStaysPublic")
}

// TestMySQLCatalogRoutesWithToken guards GHSA-fhf5-m388-4w4p on MySQL: a valid
// JWT reaches the catalog, but never the reserved credentials table.
func TestMySQLCatalogRoutesWithToken(t *testing.T) {
	base := helpers.AuthServerURL(t)
	// Seeded bcrypt user from integration/mysql/db-init.sh.
	token := helpers.LoginToken(t, base, "ada", "s3cret")

	for _, tc := range mysqlCatalogRoutes {
		// Authenticated request; access.restrict=false so items is allowed.
		// Expected: 200.
		helpers.DoAuthRequest(
			t, base+tc.path,
			nil, http.MethodGet, token, http.StatusOK,
			tc.description+" with token returns OK")
	}

	// prest_users holds credentials and is reserved.
	// Expected: 401 even for an authenticated user.
	helpers.DoAuthRequest(
		t, base+"/show/shop/shop/prest_users",
		nil, http.MethodGet, token, http.StatusUnauthorized,
		"GET /show on reserved prest_users is Unauthorized")
}

// TestMySQLRegistrySchemasListServedOnly guards the MySQL /schemas scoping:
// in registry mode only served databases are listed, never the granted but
// unserved "hidden" database (see integration/mysql/db-init.sh).
func TestMySQLRegistrySchemasListServedOnly(t *testing.T) {
	base := helpers.RegistryServerURL(t)

	// List schemas on the registry server.
	// Expected: 200 and no mention of the unserved hidden database.
	status, body := rawRequest(t, http.MethodGet, base+"/schemas", nil)
	require.Equal(t, http.StatusOK, status)
	require.NotContains(t, body, "hidden")
}
