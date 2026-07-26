package controllers

import (
	"context"
	"errors"
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
