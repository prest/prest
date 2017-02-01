package controllers

import (
	"testing"

	"net/http/httptest"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/api"
)

func TestGetDatabases(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Get databases without custom where clause", "/databases", "GET", 200},
		{"Get databases with custom where clause", "/databases?datname=$eq.prest", "GET", 200},
		{"Get databases with custom order clause", "/databases?_order=datname", "GET", 200},
		{"Get databases with custom where clause and pagination", "/databases?datname=$eq.prest&_page=1&_page_size=20", "GET", 200},
		{"Get databases with COUNT clause", "/databases?_count=", "GET", 200},
		{"Get databases with custom where invalid clause", "/databases?0datname=prest", "GET", 400},
		{"Get databases with custom where and pagination invalid", "/databases?datname=$eq.prest&_page=A", "GET", 400},
		{"Get databases with noexistent column", "/databases?datatata=$eq.test", "GET", 500},
	}

	r := api.Request{}
	router := mux.NewRouter()
	router.HandleFunc("/databases", GetDatabases).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		doRequest(t, server.URL+tc.url, r, tc.method, tc.status, "GetDatabases")
	}
}
