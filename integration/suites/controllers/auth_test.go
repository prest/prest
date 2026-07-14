package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestAuthDisable(t *testing.T) {
	base := helpers.ServerURL(t)

	// POST /auth on the default server with auth disabled.
	// Expected to fail with HTTP status NotFound because the route is unregistered.
	testutils.DoRequest(
		t, base+"/auth",
		nil, "POST", http.StatusNotFound, "AuthDisable")
}
