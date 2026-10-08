package mysql_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

// prestd-registry runs testdata/prest_mysql_registry.toml: alias "store" opens
// the shop database and alias "archive" opens other, with the same MySQL user
// (granted on both).

func TestMySQLRegistryAliasServesOwnDatabase(t *testing.T) {
	// Each alias reads tables in its own database.
	base := helpers.RegistryServerURL(t)
	testutils.DoRequest(t, base+"/store/shop/items", nil, http.MethodGet, http.StatusOK, "store reads shop", "ada")
	testutils.DoRequest(t, base+"/archive/other/tags", nil, http.MethodGet, http.StatusOK, "archive reads other")
}

func TestMySQLRegistryAliasCannotReachOtherDatabase(t *testing.T) {
	// The MySQL user can read shop, but the archive alias is pinned to other:
	// CRUD, table listing, and /show on shop through archive are all 404.
	base := helpers.RegistryServerURL(t)
	testutils.DoRequest(t, base+"/archive/shop/items", nil, http.MethodGet, http.StatusNotFound, "archive crud on shop")
	testutils.DoRequest(t, base+"/archive/shop", nil, http.MethodGet, http.StatusNotFound, "archive lists shop tables")
	testutils.DoRequest(t, base+"/show/archive/shop/items", nil, http.MethodGet, http.StatusNotFound, "archive show on shop")
}

func TestMySQLRegistryDatabasesListsAliases(t *testing.T) {
	// In registry mode /databases lists the configured aliases with their
	// physical database name, not every database the MySQL user can see.
	base := helpers.RegistryServerURL(t)
	testutils.DoRequest(t, base+"/databases", nil, http.MethodGet, http.StatusOK, "databases",
		`"datname":"store"`, `"datname":"archive"`, `"physical_name":"shop"`, `"physical_name":"other"`)
}
