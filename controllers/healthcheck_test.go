package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/testutils"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestCheckDBHealth(t *testing.T) {
	require.Nil(t, CheckDBHealth(context.Background()))
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
		checks := CheckList{tc.checkDBHealth}
		router.HandleFunc("/_health", WrappedHealthCheck(checks)).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()
		testutils.DoRequest(t, server.URL+"/_health", nil, "GET", tc.expected, "")
	}
}
