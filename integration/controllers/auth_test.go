package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
)

func TestAuthDisable(t *testing.T) {
	helpers.LoadTestConfig(t)
	r := mux.NewRouter()
	h := controllers.NewHandlersFromConfig(config.PrestConf)
	if config.PrestConf.AuthEnabled {
		r.HandleFunc("/auth", h.Auth.Login).Methods("POST")
	}
	server := httptest.NewServer(r)
	defer server.Close()

	t.Log("/auth request POST method, disable auth")
	testutils.DoRequest(t, server.URL+"/auth", nil, "POST", http.StatusNotFound, "AuthDisable")
}

func TestAuthEnable(t *testing.T) {
	helpers.LoadTestConfig(t)
	authEnabled := config.PrestConf.AuthEnabled
	defer func() { config.PrestConf.AuthEnabled = authEnabled }()
	config.PrestConf.AuthEnabled = true

	r := mux.NewRouter()
	h := controllers.NewHandlersFromConfig(config.PrestConf)
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
