package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestAuthEnable(t *testing.T) {
	base := helpers.AuthServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"GET /auth returns MethodNotAllowed when auth is enabled", "/auth", "GET", http.StatusMethodNotAllowed},
		{"POST /auth without credentials returns Unauthorized", "/auth", "POST", http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, tc.description)
	}
}
