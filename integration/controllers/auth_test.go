package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
)

func TestAuthDisable(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	cfg.AuthEnabled = false

	r := mux.NewRouter()
	h := controllers.NewHandlersFromConfig(cfg)
	server := httptest.NewServer(r)
	defer server.Close()

	t.Log("/auth request POST method, disable auth")
	testutils.DoRequest(t, server.URL+"/auth", nil, "POST", http.StatusNotFound, "AuthDisable")
	_ = h
}

func TestAuthEnable(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	cfg.AuthEnabled = true

	r := mux.NewRouter()
	h := controllers.NewHandlersFromConfig(cfg)
	r.HandleFunc("/auth", h.Auth.Login).Methods("POST")
	server := httptest.NewServer(r)
	defer server.Close()

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
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "AuthEnable")
	}
}
