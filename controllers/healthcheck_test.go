package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/testutils"
)

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
		checks := CheckList{tc.checkDBHealth}
		router := mux.NewRouter()
		h := NewHealthHandler(checks)
		router.HandleFunc("/_health", h.Handler()).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()
		testutils.DoRequest(t, server.URL+"/_health", nil, "GET", tc.expected, "")
	}
}
