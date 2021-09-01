package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/testutils"
)

func TestGetDatabases(t *testing.T) {
	config.Load()
	postgres.Load()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Get databases without custom where clause", "/databases", "GET", http.StatusOK},
		{"Get databases with custom where clause", "/databases?datname=$eq.prest", "GET", http.StatusOK},
		{"Get databases with custom order clause", "/databases?_order=datname", "GET", http.StatusOK},
		{"Get databases with custom order invalid clause", "/databases?_order=$eq.prest", "GET", http.StatusBadRequest},
		{"Get databases with custom where clause and pagination", "/databases?datname=$eq.prest&_page=1&_page_size=20", "GET", http.StatusOK},
		{"Get databases with COUNT clause", "/databases?_count=*", "GET", http.StatusOK},
		{"Get databases with custom where invalid clause", "/databases?0datname=prest", "GET", http.StatusBadRequest},
		{"Get databases with custom where and pagination invalid", "/databases?datname=$eq.prest&_page=A", "GET", http.StatusBadRequest},
		{"Get databases with noexistent column", "/databases?datatata=$eq.test", "GET", http.StatusBadRequest},
		{"Get databases with distinct", "/databases?_distinct=true", "GET", http.StatusOK},
		{"Get databases with invalid distinct", "/databases?_distinct", "GET", http.StatusOK},
	}

	router := mux.NewRouter()
	router.HandleFunc("/databases", GetDatabases).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetDatabases")
	}
}
