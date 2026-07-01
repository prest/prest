package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
	"github.com/stretchr/testify/require"
)

func TestRoutes(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	h := helpers.IntegrationHandler(t, cfg)
	require.NotNil(t, h)
}

func TestDefaultRouters(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	server := httptest.NewServer(helpers.IntegrationHandler(t, cfg))
	defer server.Close()

	var testCases = []struct {
		url    string
		method string
		status int
	}{
		{"/databases", "GET", http.StatusOK},
		{"/schemas", "GET", http.StatusOK},
		{"/_QUERIES/{queriesLocation}/{script}", "GET", http.StatusBadRequest},
		{"/{database}/{schema}", "GET", http.StatusBadRequest},
		{"/show/{database}/{schema}/{table}", "GET", http.StatusBadRequest},
		{"/{database}/{schema}/{table}", "GET", http.StatusUnauthorized},
		{"/{database}/{schema}/{table}", "POST", http.StatusUnauthorized},
		{"/batch/{database}/{schema}/{table}", "POST", http.StatusBadRequest},
		{"/{database}/{schema}/{table}", "DELETE", http.StatusUnauthorized},
		{"/{database}/{schema}/{table}", "PUT", http.StatusUnauthorized},
		{"/{database}/{schema}/{table}", "PATCH", http.StatusUnauthorized},
		{"/auth", "GET", http.StatusNotFound},
		{"/", "GET", http.StatusNotFound},
	}
	for _, tc := range testCases {
		t.Log(tc.method, "\t", tc.url)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, tc.url)
	}
}

func TestAuthRouterActive(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	cfg.AuthEnabled = true
	server := httptest.NewServer(helpers.IntegrationHandler(t, cfg))
	defer server.Close()
	testutils.DoRequest(t, server.URL+"/auth", nil, "GET", http.StatusMethodNotAllowed, "AuthEnable")
	testutils.DoRequest(t, server.URL+"/auth", nil, "POST", http.StatusUnauthorized, "AuthEnable")
}
