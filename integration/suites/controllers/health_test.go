package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestCheckDBHealth(t *testing.T) {
	base := helpers.ServerURL(t)

	// Probe the public health endpoint.
	// Expected to succeed with HTTP status OK when the database is reachable.
	testutils.DoRequest(
		t, base+"/_health",
		nil, "GET", http.StatusOK, "CheckDBHealth")
}
