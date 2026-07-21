package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mock"
	pctx "github.com/prest/prest/v2/context"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestNewAdapterSelectorMiddleware_NilRegistry(t *testing.T) {
	t.Parallel()

	var gotReq *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotReq = r
	})

	h := NewAdapterSelectorMiddleware(nil, next)
	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.NotNil(t, gotReq)
	_, ok := gotReq.Context().Value(pctx.AdapterKey).(adapters.Adapter)
	require.False(t, ok)
}

func TestAdapterSelectorMiddleware_NoRouteVars(t *testing.T) {
	// Serial: mock.New registers a sql driver (not parallel-safe).
	adapter := mock.New(t)
	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("prest-test", adapter))

	var gotReq *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotReq = r
	})

	h := NewAdapterSelectorMiddleware(registry, next)
	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.NotNil(t, gotReq)
	_, ok := gotReq.Context().Value(pctx.AdapterKey).(adapters.Adapter)
	require.False(t, ok)
}

func TestAdapterSelectorMiddleware_EmptyDatabaseVar(t *testing.T) {
	adapter := mock.New(t)
	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("prest-test", adapter))

	var gotReq *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotReq = r
	})

	h := NewAdapterSelectorMiddleware(registry, next)
	req := mux.SetURLVars(
		httptest.NewRequest(http.MethodGet, "/public/test", nil),
		map[string]string{"database": ""},
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.NotNil(t, gotReq)
	_, ok := gotReq.Context().Value(pctx.AdapterKey).(adapters.Adapter)
	require.False(t, ok)
}

func TestAdapterSelectorMiddleware_AttachesRegisteredAdapter(t *testing.T) {
	adapter := mock.New(t)
	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("prest-test", adapter))

	var gotReq *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotReq = r
	})

	h := NewAdapterSelectorMiddleware(registry, next)
	req := mux.SetURLVars(
		httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil),
		map[string]string{"database": "prest-test"},
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.NotNil(t, gotReq)
	got, ok := gotReq.Context().Value(pctx.AdapterKey).(adapters.Adapter)
	require.True(t, ok)
	require.Same(t, adapter, got)
}

func TestAdapterSelectorMiddleware_UnregisteredDatabase(t *testing.T) {
	adapter := mock.New(t)
	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("prest-test", adapter))

	var gotReq *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotReq = r
	})

	h := NewAdapterSelectorMiddleware(registry, next)
	req := mux.SetURLVars(
		httptest.NewRequest(http.MethodGet, "/missing/public/test", nil),
		map[string]string{"database": "missing"},
	)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.NotNil(t, gotReq)
	_, ok := gotReq.Context().Value(pctx.AdapterKey).(adapters.Adapter)
	require.False(t, ok)
}

func TestGetRouteVars(t *testing.T) {
	t.Parallel()

	req := mux.SetURLVars(
		httptest.NewRequest(http.MethodGet, "/db/public/t", nil),
		map[string]string{"database": "db", "schema": "public"},
	)
	vars := getRouteVars(req)
	require.Equal(t, "db", vars["database"])
	require.Equal(t, "public", vars["schema"])
}
