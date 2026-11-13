package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/v2/config"
)

func TestAllowedHostsMiddleware(t *testing.T) {
	t.Parallel()

	allowed := []string{"api.example.com", ".example.org", "127.0.0.1", "[::1]"}
	cases := []struct {
		description string
		host        string
		wantAllowed bool
	}{
		{description: "exact host", host: "api.example.com", wantAllowed: true},
		{description: "exact host with port", host: "api.example.com:3000", wantAllowed: true},
		{description: "host is matched case-insensitively", host: "API.Example.COM", wantAllowed: true},
		{description: "fully qualified host with trailing dot", host: "api.example.com.", wantAllowed: true},
		{description: "exact entry does not match a subdomain", host: "www.api.example.com", wantAllowed: false},
		{description: "exact entry does not match a lookalike suffix", host: "evilapi.example.com", wantAllowed: false},
		{description: "subdomain wildcard matches the domain itself", host: "example.org", wantAllowed: true},
		{description: "subdomain wildcard matches a subdomain", host: "www.example.org:8443", wantAllowed: true},
		{description: "subdomain wildcard matches a nested subdomain", host: "a.b.example.org", wantAllowed: true},
		{description: "subdomain wildcard does not match a lookalike domain", host: "evilexample.org", wantAllowed: false},
		{description: "IPv4 address with port", host: "127.0.0.1:3000", wantAllowed: true},
		{description: "IPv6 address with port", host: "[::1]:3000", wantAllowed: true},
		{description: "IPv6 address without port", host: "[::1]", wantAllowed: true},
		{description: "unlisted host", host: "evil.com", wantAllowed: false},
		{description: "allowed host used as a prefix of another domain", host: "api.example.com.evil.com", wantAllowed: false},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/databases", nil)
			req.Host = tc.host
			rec, called := serveMiddleware(AllowedHostsMiddleware(allowed), req)

			require.Equal(t, tc.wantAllowed, called)
			if tc.wantAllowed {
				require.Equal(t, http.StatusOK, rec.Code)
				return
			}
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.JSONEq(t, `{"error": "invalid Host header"}`, rec.Body.String())
		})
	}
}

func TestAllowedHostsMiddleware_Wildcard(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"anything.example", "10.0.0.1:3000", ""} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		_, called := serveMiddleware(AllowedHostsMiddleware([]string{"localhost", "*"}), req)
		require.True(t, called, "host %q", host)
	}
}

func TestAllowedHostsMiddleware_EmptyHostRejected(t *testing.T) {
	t.Parallel()

	// An HTTP/1.0 request may carry no Host header at all; only "*" serves it.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = ""
	rec, called := serveMiddleware(AllowedHostsMiddleware([]string{"localhost", ".example.com"}), req)

	require.False(t, called)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAllowedHostsMiddleware_EntriesAreCaseInsensitive(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "www.example.com"
	_, called := serveMiddleware(AllowedHostsMiddleware([]string{".Example.COM"}), req)

	require.True(t, called)
}

func TestNew_AllowedHosts(t *testing.T) {
	t.Parallel()

	newApp := func(cfg *config.Prest) http.Handler {
		n := New(cfg)
		r := mux.NewRouter()
		r.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte(`{"ok": true}`))
		})
		n.UseHandler(r)
		return n
	}
	serve := func(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("unset serves every host", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "evil.com"
		rec := serve(newApp(&config.Prest{}), req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("allowed host is served", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "api.example.com:3000"
		rec := serve(newApp(&config.Prest{HTTPAllowedHosts: []string{"api.example.com"}}), req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok": true}`, rec.Body.String())
	})

	t.Run("other host is rejected with a JSON error", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "evil.com"
		rec := serve(newApp(&config.Prest{HTTPAllowedHosts: []string{"api.example.com"}}), req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		require.JSONEq(t, `{"error": "invalid Host header"}`, rec.Body.String())
	})

	t.Run("CORS preflight from another host is rejected before CORS answers it", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Prest{
			HTTPAllowedHosts: []string{"api.example.com"},
			CORSAllowOrigin:  []string{"*"},
			CORSAllowMethods: []string{http.MethodGet},
		}
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		req.Host = "evil.com"
		req.Header.Set("Origin", "https://evil.com")
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		rec := serve(newApp(cfg), req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Empty(t, rec.Header().Get(headerAllowOrigin))
	})
}
