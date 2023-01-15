package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/testutils"
)

func TestCheckDBHealth(t *testing.T) {
	if err := CheckDBHealth(context.Background()); err != nil {
		t.Errorf("expected no error running the test query, got %s", err)
	}
}

func healthyDB(context.Context) error   { return nil }
func unhealthyDB(context.Context) error { return errors.New("could not connect to the database") }

func TestHealthStatus(t *testing.T) {
	for _, tc := range []struct {
		checkDBHealth func(context.Context) error
		desc          string
		expected      int
	}{
		{healthyDB, "healthy database", http.StatusOK},
		{unhealthyDB, "unhealthy database", http.StatusServiceUnavailable},
	} {
		router := mux.NewRouter()
		router.HandleFunc("/_health", WrappedHealthCheck(tc.checkDBHealth)).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()
		testutils.DoRequest(t, server.URL+"/_health", nil, "GET", tc.expected, "")
	}
}
