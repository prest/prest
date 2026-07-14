package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestQueriesServerReady(t *testing.T) {
	base := helpers.QueriesServerURL(t)

	// Probe readiness on the queries-enabled prestd.
	// Expected to succeed with HTTP status OK.
	testutils.DoRequest(
		t, base+"/_ready",
		nil, "GET", http.StatusOK, "QueriesServerReady")
}
