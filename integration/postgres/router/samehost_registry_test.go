package router_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestSameHostRegistry_AliasesRouteToOwnDatabase(t *testing.T) {
	base := helpers.SameHostServerURL(t)

	// tenant maps to secondary-db and main to prest-test, both on one host.
	// Expected: only_in_secondary is readable via tenant (200) and absent via main (404).
	testutils.DoRequest(t, base+"/tenant/public/only_in_secondary",
		nil, http.MethodGet, http.StatusOK, "SameHostRegistryTenant", `"secondary"`)
	testutils.DoRequest(t, base+"/main/public/only_in_secondary",
		nil, http.MethodGet, http.StatusNotFound, "SameHostRegistryMain")
}

func TestSameHostRegistry_PhysicalNameNotAddressable(t *testing.T) {
	base := helpers.SameHostServerURL(t)

	// Only registry aliases are served, never the physical database names.
	// Expected: 400 for both secondary-db and prest-test.
	testutils.DoRequest(t, base+"/secondary-db/public/test",
		nil, http.MethodGet, http.StatusBadRequest, "SameHostRegistryPhysicalSecondary")
	testutils.DoRequest(t, base+"/prest-test/public/test",
		nil, http.MethodGet, http.StatusBadRequest, "SameHostRegistryPhysicalPrimary")
}

func TestSameHostRegistry_WritesAreIsolated(t *testing.T) {
	base := helpers.SameHostServerURL(t)
	name := fmt.Sprintf("iso-%d", time.Now().UnixNano())
	filter := "?name=" + url.QueryEscape("$eq."+name)

	t.Cleanup(func() {
		testutils.DoRequest(t, base+"/tenant/public/test"+filter,
			nil, http.MethodDelete, http.StatusOK, "SameHostRegistryCleanup")
	})

	// Insert through the tenant alias only.
	// Expected: the row is visible through tenant and absent through main.
	testutils.DoRequest(t, base+"/tenant/public/test",
		map[string]any{"name": name}, http.MethodPost, http.StatusCreated, "SameHostRegistryInsert")

	var tenant, main []map[string]any
	testutils.DoRequestJSON(t, base+"/tenant/public/test"+filter,
		nil, http.MethodGet, http.StatusOK, "SameHostRegistryTenantRows", &tenant)
	assert.Len(t, tenant, 1)
	testutils.DoRequestJSON(t, base+"/main/public/test"+filter,
		nil, http.MethodGet, http.StatusOK, "SameHostRegistryMainRows", &main)
	assert.Empty(t, main)
}

func TestSameHostRegistry_DatabasesListsAliases(t *testing.T) {
	base := helpers.SameHostServerURL(t)

	// /databases lists registry aliases with their physical names, not host databases.
	// Expected: main, tenant and down are present; postgres is not.
	var rows []map[string]any
	testutils.DoRequestJSON(t, base+"/databases", nil, http.MethodGet, http.StatusOK, "SameHostRegistryDatabases", &rows)
	physical := map[string]any{}
	for _, r := range rows {
		if n, ok := r["datname"].(string); ok {
			physical[n] = r["physical_name"]
		}
	}
	assert.Equal(t, "prest-test", physical["main"])
	assert.Equal(t, "secondary-db", physical["tenant"])
	assert.Contains(t, physical, "down")
	assert.NotContains(t, physical, "postgres")
}

func TestSameHostRegistry_UnreachableAliasIs503(t *testing.T) {
	base := helpers.SameHostServerURL(t)

	// The down alias points at a closed port; the server still started.
	// Expected: 503 "database unavailable" without leaking the port or dial error.
	var body map[string]any
	testutils.DoRequestJSON(t, base+"/down/public/test",
		nil, http.MethodGet, http.StatusServiceUnavailable, "SameHostRegistryDown", &body)
	assert.Equal(t, "database unavailable", body["error"])
	assert.NotContains(t, fmt.Sprint(body), "6543")
	assert.NotContains(t, fmt.Sprint(body), "dial")

	// Health uses the default adapter (main), which is reachable.
	// Expected: 200.
	testutils.DoRequest(t, base+"/_health",
		nil, http.MethodGet, http.StatusOK, "SameHostRegistryHealth")
}
