// nolint
package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestSilentErrorsOnQuery(t *testing.T) {
	t.Setenv("PREST_DEBUG", "false")
	h := helpers.NewIntegrationHandlers(t)
	router := mux.NewRouter()
	router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", helpers.WithHTTPTimeout(h.Script.Execute))
	server := httptest.NewServer(router)
	defer server.Close()

	// Execute a script that fails SQL while debug is off.
	// Expected to fail with HTTP status BadRequest and a generic silent error body.
	testutils.DoRequest(
		t,
		server.URL+"/_QUERIES/error/query_w_error",
		nil,
		"GET",
		http.StatusBadRequest,
		"SilentError",
		"could not execute sql, check your prest logs",
	)
}
