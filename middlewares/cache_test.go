package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/cache"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestCacheMiddleware_Disabled(t *testing.T) {
	withPrestConf(t, &config.Prest{})

	cfg := &cache.Config{Enabled: false}
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCacheMiddleware_NonGETPassesThrough(t *testing.T) {
	withPrestConf(t, &config.Prest{})

	cfg := &cache.Config{Enabled: true}
	req := httptest.NewRequest(http.MethodPost, "/prest/public/test", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCacheMiddleware_WhitelistedURL(t *testing.T) {
	withPrestConf(t, &config.Prest{JWTWhiteList: []string{`\/auth`}})

	cfg := &cache.Config{Enabled: true}
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCacheMiddleware_NoEndpointRule(t *testing.T) {
	withPrestConf(t, &config.Prest{})

	cfg := &cache.Config{
		Enabled: true,
		Endpoints: []cache.Endpoint{
			{Enabled: true, Endpoint: "/other", Time: 5},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCacheMiddleware_MatchURLError(t *testing.T) {
	withPrestConf(t, &config.Prest{JWTWhiteList: []string{"[invalid"}})

	cfg := &cache.Config{Enabled: true}
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(CacheMiddleware(cfg), req)

	require.False(t, called)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
