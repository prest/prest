package middlewares

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"

	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
)

const testRequestIDHeader = "X-Request-Id"

var generatedRequestID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// serveRequestID runs req through RequestIDMiddleware and returns the
// response and the request the next handler received.
func serveRequestID(t *testing.T, req *http.Request) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	rec := httptest.NewRecorder()
	var got *http.Request
	RequestIDMiddleware(testRequestIDHeader).ServeHTTP(rec, req, func(_ http.ResponseWriter, r *http.Request) {
		got = r
	})
	require.NotNil(t, got, "next handler was not called")
	return rec, got
}

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/databases", nil)
	rec, got := serveRequestID(t, req)

	id := rec.Header().Get(testRequestIDHeader)
	require.Regexp(t, generatedRequestID, id)
	require.Equal(t, id, got.Header.Get(testRequestIDHeader))
	require.Equal(t, id, got.Context().Value(pctx.RequestIDKey))
	require.Equal(t, id, RequestIDFromContext(got.Context()))
}

func TestRequestIDMiddleware_GeneratesDistinctIDs(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for range 100 {
		rec, _ := serveRequestID(t, httptest.NewRequest(http.MethodGet, "/", nil))
		id := rec.Header().Get(testRequestIDHeader)
		require.False(t, seen[id], "request ID %q generated twice", id)
		seen[id] = true
	}
}

func TestRequestIDMiddleware_KeepsClientID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		description string
		id          string
	}{
		{description: "UUID", id: "3f2b8e0c-9c1d-4f5e-8a7b-6c5d4e3f2a1b"},
		{description: "single character", id: "a"},
		{description: "every printable ASCII symbol", id: `!"#$%&'()*+,-./:;<=>?@[\]^_{|}~` + "`"},
		{description: "longest accepted ID", id: strings.Repeat("a", maxRequestIDLength)},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(testRequestIDHeader, tc.id)
			rec, got := serveRequestID(t, req)

			require.Equal(t, tc.id, rec.Header().Get(testRequestIDHeader))
			require.Equal(t, tc.id, got.Header.Get(testRequestIDHeader))
			require.Equal(t, tc.id, RequestIDFromContext(got.Context()))
		})
	}
}

func TestRequestIDMiddleware_ReplacesInvalidClientID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		description string
		id          string
	}{
		{description: "too long", id: strings.Repeat("a", maxRequestIDLength+1)},
		{description: "contains a space", id: "abc def"},
		{description: "contains a tab", id: "abc\tdef"},
		{description: "contains a control character", id: "abc\x00def"},
		{description: "contains non-ASCII text", id: "pedido-é"},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header[testRequestIDHeader] = []string{tc.id}
			rec, got := serveRequestID(t, req)

			id := rec.Header().Get(testRequestIDHeader)
			require.Regexp(t, generatedRequestID, id)
			require.Equal(t, id, got.Header.Get(testRequestIDHeader))
			require.Equal(t, id, RequestIDFromContext(got.Context()))
		})
	}
}

func TestRequestIDMiddleware_DoesNotModifyIncomingRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, got := serveRequestID(t, req)

	require.NotEmpty(t, got.Header.Get(testRequestIDHeader))
	require.Empty(t, req.Header.Get(testRequestIDHeader))
	require.Empty(t, RequestIDFromContext(req.Context()))
}

func TestRequestIDFromContext_Unset(t *testing.T) {
	t.Parallel()

	require.Empty(t, RequestIDFromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context()))
}

func TestBaseStack_RequestIDInAccessLog(t *testing.T) {
	t.Parallel()

	stack := baseStack(60, testRequestIDHeader)
	var buf bytes.Buffer
	for _, h := range stack {
		if l, ok := h.(*negroni.Logger); ok {
			l.ALogger = log.New(&buf, "", 0)
		}
	}
	n := negroni.New(stack...)
	n.UseHandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{}`))
	})

	req := httptest.NewRequest(http.MethodGet, "/databases", nil)
	req.Header.Set(testRequestIDHeader, "req-42")
	rec := httptest.NewRecorder()
	n.ServeHTTP(rec, req)

	require.Equal(t, "req-42", rec.Header().Get(testRequestIDHeader))
	line := strings.TrimSpace(buf.String())
	require.True(t, strings.HasSuffix(line, "| GET /databases | req-42"), line)
}

func TestBaseStack_NoRequestIDByDefault(t *testing.T) {
	t.Parallel()

	n := negroni.New(BaseStack(60)...)
	n.UseHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, RequestIDFromContext(r.Context()))
		w.Write([]byte(`{}`))
	})
	rec := httptest.NewRecorder()
	n.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get(testRequestIDHeader))
}

func TestNew_RequestID(t *testing.T) {
	t.Parallel()

	newApp := func(cfg *config.Prest) http.Handler {
		n := New(cfg)
		r := mux.NewRouter()
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"request_id": "` + RequestIDFromContext(r.Context()) + `"}`))
		})
		n.UseHandler(r)
		return n
	}
	serve := func(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("unset sends no request ID", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(testRequestIDHeader, "client-id")
		rec := serve(newApp(&config.Prest{}), req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, rec.Header().Get(testRequestIDHeader))
		require.JSONEq(t, `{"request_id": ""}`, rec.Body.String())
	})

	t.Run("handlers see the ID sent back", func(t *testing.T) {
		t.Parallel()

		rec := serve(newApp(&config.Prest{HTTPRequestIDHeader: testRequestIDHeader}),
			httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		id := rec.Header().Get(testRequestIDHeader)
		require.Regexp(t, generatedRequestID, id)
		require.JSONEq(t, `{"request_id": "`+id+`"}`, rec.Body.String())
	})

	t.Run("error responses carry the ID", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Prest{
			HTTPRequestIDHeader: testRequestIDHeader,
			HTTPAllowedHosts:    []string{"api.example.com"},
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "evil.com"
		req.Header.Set(testRequestIDHeader, "client-id")
		rec := serve(newApp(cfg), req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, "client-id", rec.Header().Get(testRequestIDHeader))
	})

	t.Run("CORS exposes the request ID header", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Prest{
			HTTPRequestIDHeader: testRequestIDHeader,
			CORSAllowOrigin:     []string{"*"},
			CORSAllowMethods:    []string{http.MethodGet},
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := serve(newApp(cfg), req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, testRequestIDHeader, rec.Header().Get("Access-Control-Expose-Headers"))
	})

	t.Run("CORS exposes nothing without a request ID header", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Prest{
			CORSAllowOrigin:  []string{"*"},
			CORSAllowMethods: []string{http.MethodGet},
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://app.example.com")
		rec := serve(newApp(cfg), req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, rec.Header().Get("Access-Control-Expose-Headers"))
	})
}
