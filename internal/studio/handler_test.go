package studio

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prest/prest/v2/helpers"
	"github.com/stretchr/testify/require"
)

func TestHandlerDisabled(t *testing.T) {
	t.Parallel()
	h := Handler(false)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_studio/", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlerRedirectRoot(t *testing.T) {
	t.Parallel()
	h := Handler(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_studio", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "/_studio/", rec.Header().Get("Location"))
}

func TestHandlerIndex(t *testing.T) {
	t.Parallel()
	h := Handler(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_studio/", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Contains(t, rec.Body.String(), "pREST Studio")
}

func TestHandlerSPAFallback(t *testing.T) {
	t.Parallel()
	h := Handler(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_studio/data", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	require.Contains(t, rec.Body.String(), "pREST Studio")
}

func TestHandlerAPINotSwallowed(t *testing.T) {
	t.Parallel()
	h := Handler(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_studio/api/unknown", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandlerMeta(t *testing.T) {
	t.Parallel()
	helpers.Version = "test-ver"
	helpers.Commit = "abc123"
	helpers.Date = "2026-01-01"
	t.Cleanup(func() {
		helpers.Version = ""
		helpers.Commit = ""
		helpers.Date = ""
	})

	h := Handler(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_studio/api/meta", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var meta Meta
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	require.Equal(t, "test-ver", meta.Version)
	require.Equal(t, "abc123", meta.Commit)
	require.Equal(t, "2026-01-01", meta.BuildDate)
	require.Equal(t, "/", meta.APIBasePath)
	require.Equal(t, "/_mcp", meta.MCPEndpoint)

	body := rec.Body.String()
	require.NotContains(t, strings.ToLower(body), "password")
	require.NotContains(t, strings.ToLower(body), "jwt")
	require.NotContains(t, strings.ToLower(body), "secret")
}

func TestHandlerTraversal(t *testing.T) {
	t.Parallel()
	h := Handler(true)
	cases := []string{
		"/_studio/../etc/passwd",
		"/_studio/%2e%2e/etc/passwd",
	}
	for _, p := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, p, nil)
		h.ServeHTTP(rec, req)
		require.NotEqual(t, http.StatusOK, rec.Code, p)
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	t.Parallel()
	h := Handler(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/_studio/", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHandlerSecurityHeaders(t *testing.T) {
	t.Parallel()
	h := Handler(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_studio/", nil)
	h.ServeHTTP(rec, req)
	require.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
	require.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
}

func TestHandlerHead(t *testing.T) {
	t.Parallel()
	h := Handler(true)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/_studio/", nil)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	_ = body
}
