package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
)

func TestCheckDBHealth(t *testing.T) {
	h := helpers.NewIntegrationHandlers(t)
	r := mux.NewRouter()
	r.HandleFunc("/_health", helpers.WithHTTPTimeout(h.Health.Handler())).Methods("GET")
	server := httptest.NewServer(r)
	defer server.Close()

	testutils.DoRequest(t, server.URL+"/_health", nil, "GET", http.StatusOK, "CheckDBHealth")
}
