package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/cache"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/stretchr/testify/require"
)

func TestCacheMiddleware_Disabled(t *testing.T) {
	t.Parallel()

	cfg := &cache.Config{Enabled: false}
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg, nil), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCacheMiddleware_NonGETPassesThrough(t *testing.T) {
	t.Parallel()

	cfg := &cache.Config{Enabled: true}
	req := httptest.NewRequest(http.MethodPost, "/prest/public/test", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg, nil), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCacheMiddleware_WhitelistedURL(t *testing.T) {
	t.Parallel()

	cfg := &cache.Config{Enabled: true}
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg, []string{`\/auth`}), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCacheMiddleware_NoEndpointRule(t *testing.T) {
	t.Parallel()

	cfg := &cache.Config{
		Enabled: true,
		Endpoints: []cache.Endpoint{
			{Enabled: true, Endpoint: "/other", Time: 5},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg, nil), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCacheMiddleware_MatchURLError(t *testing.T) {
	t.Parallel()

	cfg := &cache.Config{Enabled: true}
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg, []string{"[invalid"}), req)

	require.False(t, called)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestCacheMiddleware_CacheLookup(t *testing.T) {
	t.Parallel()

	const path = "/prest/public/test"
	newCfg := func(t *testing.T) *cache.Config {
		t.Helper()
		return &cache.Config{
			Enabled:     true,
			Time:        5,
			StoragePath: t.TempDir(),
			Endpoints: []cache.Endpoint{
				{Enabled: true, Endpoint: path, Time: 5},
			},
		}
	}

	t.Run("hit", func(t *testing.T) {
		cfg := newCfg(t)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		cfg.BuntSet(CacheKey(req), `[{"cached":true}]`)

		rec, called := serveMiddleware(CacheMiddleware(cfg, nil), req)

		require.False(t, called)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "prestd", rec.Header().Get("Cache-Server"))
		require.JSONEq(t, `[{"cached":true}]`, rec.Body.String())
	})

	t.Run("miss", func(t *testing.T) {
		cfg := newCfg(t)

		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec, called := serveMiddleware(CacheMiddleware(cfg, nil), req)

		require.True(t, called)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, rec.Header().Get("Cache-Server"))
	})

	// A cache entry populated under one user's identity must not be served to
	// a different user hitting the identical URL: FieldsPermissions can differ
	// per user, and the handler that would recompute it never runs on a hit.
	t.Run("does not leak across users on the same URL", func(t *testing.T) {
		cfg := newCfg(t)
		userAReq := httptest.NewRequest(http.MethodGet, path, nil)
		userAReq = userAReq.WithContext(withUser(userAReq.Context(), auth.User{Username: "alice"}))
		cfg.BuntSet(CacheKey(userAReq), `[{"ssn":"redacted-for-alice-only"}]`)

		userBReq := httptest.NewRequest(http.MethodGet, path, nil)
		userBReq = userBReq.WithContext(withUser(userBReq.Context(), auth.User{Username: "bob"}))
		rec, called := serveMiddleware(CacheMiddleware(cfg, nil), userBReq)

		require.True(t, called, "bob must recompute the response, not receive alice's cached one")
		require.Empty(t, rec.Header().Get("Cache-Server"))
	})
}

// TestCacheKey_rawQueryCannotForgeIdentity guards against key collisions via
// RawQuery: Go preserves an unescaped "?" in it, so URL.String() can echo the
// literal "__prest_user=<name>" suffix pREST itself appends. The identity
// suffix must be a digest of the authenticated username, not the raw value,
// so embedding another user's name in RawQuery can't reproduce their key.
func TestCacheKey_rawQueryCannotForgeIdentity(t *testing.T) {
	t.Parallel()

	victimReq := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	victimReq = victimReq.WithContext(withUser(victimReq.Context(), auth.User{Username: "admin"}))
	victimKey := CacheKey(victimReq)

	forgedPath := "/prest/public/test?__prest_user=admin"
	attackerReq := httptest.NewRequest(http.MethodGet, forgedPath, nil)
	attackerReq = attackerReq.WithContext(withUser(attackerReq.Context(), auth.User{Username: "eve"}))

	require.NotEqual(t, victimKey, CacheKey(attackerReq))
}
