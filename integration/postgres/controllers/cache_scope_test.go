package controllers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/stretchr/testify/require"
)

// TestCache_DoesNotLeakAcrossIdentities guards the GET response cache
// (middlewares.CacheMiddleware) leaking a response computed under one user's
// FieldsPermissions to a different identity requesting the identical URL. The
// cache sits downstream of AccessControl but upstream of the handler's own
// per-user column filtering, so on a cache hit that filtering never runs
// again — a cache key scoped only by URL would let a more-privileged user's
// cached response (extra columns) leak to a less-privileged one.
//
// testdata/prest_cache.toml grants test@postgres.rest an extra "celphone"
// column on test6 that cache-scope-user-b (no per-user override, falls back
// to the generic table permissions) doesn't get. test@postgres.rest reads
// first, populating the cache under their identity; the very next request
// from cache-scope-user-b to the identical URL must recompute its own
// restricted response, never reuse the more-privileged cached one. auth is
// mandatory on this server, so both sides must be real logged-in identities
// (an anonymous request is rejected by AuthMiddleware before it ever reaches
// AccessControl or the cache).
func TestCache_DoesNotLeakAcrossIdentities(t *testing.T) {
	base := helpers.CacheServerURL(t)
	tokenA := helpers.LoginToken(t, base, "test@postgres.rest", "123456")
	tokenB := helpers.LoginToken(t, base, "cache-scope-user-b@postgres.rest", "123456")
	url := base + "/prest-test/public/test6"

	// More-privileged read: gets the expanded field set, including "celphone".
	// This is the response that ends up cached under this user's identity.
	rowsA := doCacheJSONRequest(t, url, "Bearer "+tokenA)
	require.NotEmpty(t, rowsA)
	require.Contains(t, rowsA[0], "celphone", "test@postgres.rest should see celphone per prest_cache.toml")

	// A different, less-privileged identity reading the identical URL right
	// after: must recompute under its own (restricted) permission set, not
	// reuse the more-privileged user's cached response.
	rowsB := doCacheJSONRequest(t, url, "Bearer "+tokenB)
	require.NotEmpty(t, rowsB)
	require.NotContains(t, rowsB[0], "celphone", "cache-scope-user-b must not see a column only test@postgres.rest is permitted")
}

func doCacheJSONRequest(t *testing.T, url, authHeader string) []map[string]any {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var rows []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&rows))
	return rows
}
