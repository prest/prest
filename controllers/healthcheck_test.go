package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/testutils"
)

func init() {
	dbConn = DbConnection{}
}

func TestHealthStatus(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Healthcheck endpoint", "/_health", "GET", http.StatusOK},
	}

	router := mux.NewRouter()
	router.HandleFunc("/_health", HealthStatus).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "HealthStatus")
		})
	}
}
