package middlewares

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
)

// newGuardTestStack builds the full pREST middleware stack with the given
// guard config mutation applied.
func newGuardTestStack(t *testing.T, mutate func(*config.GuardConf)) *negroni.Negroni {
	t.Helper()
	cfg := &config.Prest{}
	if mutate != nil {
		mutate(&cfg.Guard)
	}
	return New(cfg)
}

// serveGuardRequest sends a GET against the stack and records the response.
func serveGuardRequest(n *negroni.Negroni, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	n.ServeHTTP(rec, req)
	return rec
}

// newGuardOnlyStack builds a stack with just the guard handler in front of a
// stub terminal handler that answers 200 and echoes the request body, so
// tests assert guard behavior without the rest of the pREST stack.
func newGuardOnlyStack(t *testing.T, mutate func(*config.GuardConf)) *negroni.Negroni {
	t.Helper()
	conf := config.GuardConf{}
	if mutate != nil {
		mutate(&conf)
	}
	handler, err := GuardMiddleware(conf)
	require.NoError(t, err)
	return negroni.New(handler, negroni.HandlerFunc(func(
		w http.ResponseWriter, r *http.Request, _ http.HandlerFunc,
	) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
}

// captureSlog redirects the default slog logger into a buffer for the
// duration of the test.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

// TestGuardDisabledByDefault verifies no engine is built unless guard.enabled
// is explicitly turned on.
func TestGuardDisabledByDefault(t *testing.T) {
	n := newGuardTestStack(t, nil)
	// With guard disabled even an obvious injection attempt must pass through
	// untouched: no engine is built and behavior is byte-identical to before.
	res := serveGuardRequest(n, "/teste?name=1%27%20OR%20%271%27%3D%271")
	require.Equal(t, http.StatusOK, res.Code)
}

// TestGuardEnabledBlocksInjection verifies query injection payloads are
// rejected with 400 while clean requests pass.
func TestGuardEnabledBlocksInjection(t *testing.T) {
	n := newGuardTestStack(t, func(g *config.GuardConf) {
		g.Enabled = true
	})

	res := serveGuardRequest(n, "/teste?name=1%27%20OR%20%271%27%3D%271")
	// Penetration detection answers 400 Bad Request for matched payloads.
	require.Equal(t, http.StatusBadRequest, res.Code)

	res = serveGuardRequest(n, "/teste?name=alice")
	require.Equal(t, http.StatusOK, res.Code)
}

// TestGuardPassiveModeLogsButDoesNotBlock verifies passive preview logs the
// block through the OnBlock hook without rejecting the request.
func TestGuardPassiveModeLogsButDoesNotBlock(t *testing.T) {
	logs := captureSlog(t)
	n := newGuardTestStack(t, func(g *config.GuardConf) {
		g.Enabled = true
		g.Passive = true
	})

	res := serveGuardRequest(n, "/teste?name=1%27%20OR%20%271%27%3D%271")
	require.Equal(t, http.StatusOK, res.Code)
	// Passive preview must actually emit "would have blocked" logs.
	require.Contains(t, logs.String(), "guard request blocked")
	require.Contains(t, logs.String(), "passive=true")
}

// TestGuardRateLimitPerClient verifies the per-client rate limit returns 429
// once the configured request count is exceeded.
func TestGuardRateLimitPerClient(t *testing.T) {
	n := newGuardTestStack(t, func(g *config.GuardConf) {
		g.Enabled = true
		g.RateLimit = 2
		g.RateLimitWindow = 60
	})

	for i := 0; i < 2; i++ {
		res := serveGuardRequest(n, "/teste")
		require.Equal(t, http.StatusOK, res.Code, "request %d within limit", i+1)
	}
	res := serveGuardRequest(n, "/teste")
	require.Equal(t, http.StatusTooManyRequests, res.Code)
}

// TestGuardBlacklistBlockedWhitelistAllowed verifies blacklisted CIDRs are
// rejected with 403.
func TestGuardBlacklistBlockedWhitelistAllowed(t *testing.T) {
	n := newGuardTestStack(t, func(g *config.GuardConf) {
		g.Enabled = true
		g.Blacklist = []string{"203.0.113.0/24"}
	})

	req := httptest.NewRequest(http.MethodGet, "/teste", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	rec := httptest.NewRecorder()
	n.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestGuardExcludePathsSkipsAllChecks verifies excluded paths bypass every
// guard check, including IP policy and rate limits, via the prest-side
// short-circuit.
func TestGuardExcludePathsSkipsAllChecks(t *testing.T) {
	n := newGuardTestStack(t, func(g *config.GuardConf) {
		g.Enabled = true
		g.Blacklist = []string{"203.0.113.0/24"}
		g.ExcludePaths = []string{"/health"}
	})

	// exclude_paths promises "skips guard checks": even IP policy must not
	// apply on excluded paths and their subtrees.
	for _, target := range []string{"/health", "/health/ready"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.RemoteAddr = "203.0.113.7:54321"
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "path %s", target)
	}

	// The same blocked IP is still rejected on regular paths.
	req := httptest.NewRequest(http.MethodGet, "/teste", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	rec := httptest.NewRecorder()
	n.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	// Subtree matching is exact: "/healthcheck" is not "/health".
	req = httptest.NewRequest(http.MethodGet, "/healthcheck", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	rec = httptest.NewRecorder()
	n.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestGuardInvalidConfigFailsClosed verifies an unusable engine config answers
// 500 on every request instead of serving unguarded.
func TestGuardInvalidConfigFailsClosed(t *testing.T) {
	n := newGuardTestStack(t, func(g *config.GuardConf) {
		g.Enabled = true
		g.BlockCloudProviders = []string{"not-a-provider"}
	})

	res := serveGuardRequest(n, "/teste")
	require.Equal(t, http.StatusInternalServerError, res.Code)
}

// TestGuardInvalidEnabledFailsClosed verifies a non-boolean guard.enabled
// carried on GuardConf.Invalid mounts the fail-closed middleware.
func TestGuardInvalidEnabledFailsClosed(t *testing.T) {
	// config.Parse cannot return errors, so a non-boolean guard.enabled is
	// carried on GuardConf.Invalid: New() must mount the 500 middleware even
	// though Enabled itself parsed as false.
	n := newGuardTestStack(t, func(g *config.GuardConf) {
		g.Invalid = errors.New(`guard.enabled: "yes" is not a valid boolean`)
	})

	res := serveGuardRequest(n, "/teste")
	require.Equal(t, http.StatusInternalServerError, res.Code)
}

// TestGuardTrustedProxyResolution covers X-Forwarded-For handling for trusted
// and untrusted peers, including the rightmost-untrusted hop rule.
func TestGuardTrustedProxyResolution(t *testing.T) {
	t.Run("untrusted peer ignores forwarded header", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			g.TrustedProxies = []string{"10.0.0.9"}
			g.Blacklist = []string{"1.2.3.4/32"}
		})
		req := httptest.NewRequest(http.MethodGet, "/teste", nil)
		req.RemoteAddr = "203.0.113.7:54321"
		// Spoofed header from an untrusted peer must be ignored entirely.
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("trusted peer resolves client from forwarded header", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			g.TrustedProxies = []string{"10.0.0.9"}
			g.Blacklist = []string{"1.2.3.4/32"}
		})
		req := httptest.NewRequest(http.MethodGet, "/teste", nil)
		req.RemoteAddr = "10.0.0.9:54321"
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("rightmost untrusted hop wins", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			g.TrustedProxies = []string{"10.0.0.9", "10.0.0.8"}
			g.Blacklist = []string{"1.2.3.4/32"}
		})
		req := httptest.NewRequest(http.MethodGet, "/teste", nil)
		req.RemoteAddr = "10.0.0.9:54321"
		// 10.0.0.9 and 10.0.0.8 are trusted, so the client is 1.2.3.4.
		req.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.8, 10.0.0.9")
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("forwarded header split across lines joins hops", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			g.TrustedProxies = []string{"10.0.0.9", "10.0.0.8"}
			g.Blacklist = []string{"1.2.3.4/32"}
		})
		req := httptest.NewRequest(http.MethodGet, "/teste", nil)
		req.RemoteAddr = "10.0.0.9:54321"
		// A proxy that appends a second X-Forwarded-For header line instead of
		// joining its hop into the existing value: the spoofable 9.9.9.9 must
		// not shadow the real client 1.2.3.4 recorded by the trusted proxy.
		req.Header["X-Forwarded-For"] = []string{"9.9.9.9", "1.2.3.4, 10.0.0.9"}
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("unlisted client behind trusted proxy is allowed", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			g.TrustedProxies = []string{"10.0.0.9"}
			g.Blacklist = []string{"1.2.3.4/32"}
		})
		req := httptest.NewRequest(http.MethodGet, "/teste", nil)
		req.RemoteAddr = "10.0.0.9:54321"
		req.Header.Set("X-Forwarded-For", "198.51.100.7")
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

// TestGuardBodyScan covers the bounded body pre-scan: enforcing mode rejects
// body-only attack payloads with 400, clean bodies pass through intact, and
// passive mode logs the threat without rejecting.
func TestGuardBodyScan(t *testing.T) {
	t.Run("enforcing blocks threat in body", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {})
		req := httptest.NewRequest(http.MethodPost, "/teste",
			strings.NewReader(`{"name": "1' OR '1'='1"}`))
		req.RemoteAddr = "198.51.100.7:54321"
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, "Suspicious activity detected", rec.Body.String())
	})

	t.Run("enforcing passes clean body through intact", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			// Tiny cap so the echo handler proves bytes beyond the scan cap
			// are restored for downstream handlers.
			g.MaxBodyBytes = 8
		})
		payload := `abcdefgh-"harmless"`
		req := httptest.NewRequest(http.MethodPost, "/teste", strings.NewReader(payload))
		req.RemoteAddr = "198.51.100.7:54321"
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, payload, rec.Body.String())
	})

	t.Run("passive logs and passes threat in body", func(t *testing.T) {
		logs := captureSlog(t)
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			g.Passive = true
		})
		req := httptest.NewRequest(http.MethodPost, "/teste",
			strings.NewReader(`{"name": "1' OR '1'='1"}`))
		req.RemoteAddr = "198.51.100.7:54321"
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, logs.String(), "guard request blocked")
		require.Contains(t, logs.String(), "check=suspicious_activity")
	})

	t.Run("whitelisted client skips body scan", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			g.Whitelist = []string{"198.51.100.7"}
		})
		req := httptest.NewRequest(http.MethodPost, "/teste",
			strings.NewReader(`{"name": "1' OR '1'='1"}`))
		req.RemoteAddr = "198.51.100.7:54321"
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, `{"name": "1' OR '1'='1"}`, rec.Body.String())
	})

	t.Run("non-whitelisted client denied by engine before body scan", func(t *testing.T) {
		n := newGuardOnlyStack(t, func(g *config.GuardConf) {
			g.Whitelist = []string{"198.51.100.7"}
		})
		req := httptest.NewRequest(http.MethodPost, "/teste",
			strings.NewReader(`{"name": "1' OR '1'='1"}`))
		req.RemoteAddr = "198.51.100.8:54321"
		rec := httptest.NewRecorder()
		n.ServeHTTP(rec, req)
		// With a whitelist configured the engine denies everyone else outright,
		// so the body scan's 400 must not preempt the engine's 403.
		require.Equal(t, http.StatusForbidden, rec.Code)
	})
}

// TestGuardEngineRunsBeforeBodyScan verifies the engine's verdicts take
// precedence over the body scan: over-limit clients get the engine's 429 and
// blacklisted peers get the engine's 403 even when the body also carries an
// attack payload.
func TestGuardEngineRunsBeforeBodyScan(t *testing.T) {
	n := newGuardOnlyStack(t, func(g *config.GuardConf) {
		g.RateLimit = 1
		g.RateLimitWindow = 60
	})

	// The first request consumes the single-request rate limit budget.
	res := serveGuardRequest(n, "/teste")
	require.Equal(t, http.StatusOK, res.Code)

	// The next request carries a body-only payload: the engine's 429 must win
	// over the body scan's 400.
	req := httptest.NewRequest(http.MethodPost, "/teste",
		strings.NewReader(`{"name": "1' OR '1'='1"}`))
	req.RemoteAddr = "192.0.2.1:1234"
	rec := httptest.NewRecorder()
	n.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	// A blacklisted peer with an attack body gets the engine's 403, not the
	// body scan's 400.
	n = newGuardOnlyStack(t, func(g *config.GuardConf) {
		g.Blacklist = []string{"198.51.100.0/24"}
	})
	req = httptest.NewRequest(http.MethodPost, "/teste",
		strings.NewReader(`{"name": "1' OR '1'='1"}`))
	req.RemoteAddr = "198.51.100.7:54321"
	rec = httptest.NewRecorder()
	n.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}
