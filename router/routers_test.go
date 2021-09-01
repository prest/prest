package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/testutils"
)

func init() {
	// load configuration at the beginning of the tests
	config.Load()
	postgres.Load()
}

func TestInitRouter(t *testing.T) {
	initRouter()
	if router == nil {
		t.Errorf("Router should not be nil.")
	}
}

func TestRoutes(t *testing.T) {
	router = nil
	r := Routes()
	if r == nil {
		t.Errorf("Should return a router.")
	}
}

func TestDefaultRouters(t *testing.T) {
	server := httptest.NewServer(GetRouter())
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
		{"/", "GET", http.StatusNotFound},
	}
	for _, tc := range testCases {
		t.Log(tc.method, "\t", tc.url)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, tc.url)
	}
}
