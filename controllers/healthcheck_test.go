package controllers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func healthyDB(context.Context) error   { return nil }
func unhealthyDB(context.Context) error { return errors.New("could not connect to the database") }

func TestHealthStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		checkDBHealth func(context.Context) error
		desc          string
		expected      int
	}{
		{healthyDB, "healthy database", http.StatusOK},
		{unhealthyDB, "unhealthy database", http.StatusServiceUnavailable},
	} {
		checks := CheckList{tc.checkDBHealth}
		router := mux.NewRouter()
		h := NewHealthHandler(checks)
		router.HandleFunc("/_health", h.Handler()).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()

		resp, err := http.Get(server.URL + "/_health")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, tc.expected, resp.StatusCode)
	}
}

func TestHealthHandler_AdapterName(t *testing.T) {
	t.Parallel()

	mysql := NewHealthHandler(CheckList{healthyDB})
	mysql.adapterName = "mysql"
	rec := httptest.NewRecorder()
	mysql.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_health", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Equal(t, `{"adapter":"mysql"}`, rec.Body.String())

	empty := NewHealthHandler(CheckList{healthyDB})
	rec = httptest.NewRecorder()
	empty.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_health", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	require.Empty(t, body)

	down := NewHealthHandler(CheckList{unhealthyDB})
	down.adapterName = "mysql"
	rec = httptest.NewRecorder()
	down.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_health", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"error":"database unavailable"}`, rec.Body.String())
}

func TestReadyStatus(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		check func(context.Context) error
		want  int
		desc  string
	}{
		{healthyDB, http.StatusOK, "all databases ready"},
		{unhealthyDB, http.StatusServiceUnavailable, "database unavailable"},
	} {
		h := NewHealthHandler(CheckList{tc.check})
		router := mux.NewRouter()
		router.HandleFunc("/_ready", h.Handler()).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()

		resp, err := http.Get(server.URL + "/_ready")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, tc.want, resp.StatusCode)
	}
}

// A failing check answers 503 with a JSON body so clients and probes can tell
// an outage from an empty response; the driver detail stays in the logs.
func TestHealthHandler_UnavailableBody(t *testing.T) {
	t.Parallel()

	h := NewHealthHandler(CheckList{unhealthyDB})
	req := httptest.NewRequest(http.MethodGet, "/_health", nil)
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.JSONEq(t, `{"error":"database unavailable"}`, rec.Body.String())
}
