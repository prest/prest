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

func TestGetSchemas(t *testing.T) {
	config.Load()
	postgres.Load()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        string
	}{
		{"Get schemas without custom where clause", "/schemas", "GET", http.StatusOK, "[{\"schema_name\": \"information_schema\"}, {\"schema_name\": \"pg_catalog\"}, {\"schema_name\": \"pg_toast\"}, {\"schema_name\": \"public\"}]"},
		{"Get schemas with custom where clause", "/schemas?schema_name=$eq.public", "GET", http.StatusOK, "[{\"schema_name\": \"public\"}]"},
		{"Get schemas with custom order clause", "/schemas?schema_name=$eq.public&_order=schema_name", "GET", http.StatusOK, "[{\"schema_name\": \"public\"}]"},
		{"Get schemas with custom order invalid clause", "/schemas?schema_name=$eq.public&_order=$eq.schema_name", "GET", http.StatusBadRequest, "invalid identifier\n"},
		{"Get schemas with custom where clause and pagination", "/schemas?schema_name=$eq.public&_page=1&_page_size=20", "GET", http.StatusOK, "[{\"schema_name\": \"public\"}]"},
		{"Get schemas with COUNT clause", "/schemas?_count=*", "GET", http.StatusOK, "[{\"count\": 4}]"},
		{"Get schemas with custom where invalid clause", "/schemas?0schema_name=$eq.public", "GET", http.StatusBadRequest, "0schema_name: invalid identifier\n"},
		{"Get schemas with noexistent column", "/schemas?schematame=$eq.test", "GET", http.StatusBadRequest, "pq: column \"schematame\" does not exist\n"},
		{"Get schemas with distinct clause", "/schemas?schema_name=$eq.public&_distinct=true", "GET", http.StatusOK, "[{\"schema_name\": \"public\"}]"},
	}

	router := mux.NewRouter()
	router.HandleFunc("/schemas", GetSchemas).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetSchemas", tc.body)
	}
}

func TestVersionDependentGetSchemas(t *testing.T) {
	//This test supports error messages from different versions of Go.
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        string
	}{
		{"Get schemas with custom where and pagination invalid",
			"/schemas?schema_name=$eq.public&_page=A",
			"GET",
			http.StatusBadRequest,
			"strconv.Atoi: parsing \"A\": invalid syntax\n",
		},
	}

	router := mux.NewRouter()
	router.HandleFunc("/schemas", GetSchemas).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetSchemas", tc.body)
	}

}
