package controllers

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/testutils"
)

func healthyDB() error   { return nil }
func unhealthyDB() error { return errors.New("could not connect to the database") }

func TestHealthStatus(t *testing.T) {
	for _, tc := range []struct {
		checkDBHealth func() error
		desc          string
		expected      int
		body          string
	}{
		{healthyDB, "healthy database", 200, "ok"},
		{unhealthyDB, "unhealthy database", 503, "unable to run queries on the database"},
	} {
		router := mux.NewRouter()
		router.HandleFunc("/_health", WrappedHealthCheck(tc.checkDBHealth)).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()
		testutils.DoRequest(t, server.URL+"/_health", nil, "GET", tc.expected, tc.body)
	}
}
