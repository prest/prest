package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestAuthDisable(t *testing.T) {
	base := helpers.ServerURL(t)
	t.Log("/auth request POST method, disable auth")
	testutils.DoRequest(t, base+"/auth", nil, "POST", http.StatusNotFound, "AuthDisable")
}

func TestAuthEnable(t *testing.T) {
	base := helpers.AuthServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"/auth request GET method", "/auth", "GET", http.StatusMethodNotAllowed},
		{"/auth request POST method", "/auth", "POST", http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, "AuthEnable")
	}
}
