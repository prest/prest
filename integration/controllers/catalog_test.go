package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
)

func TestGetDatabases(t *testing.T) {
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

	h := helpers.NewIntegrationHandlers(t)
	router := mux.NewRouter()
	router.HandleFunc("/databases", h.Catalog.ListDatabases).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetDatabases")
	}
}

func TestGetSchemas(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        []string
	}{
		{"Get schemas without custom where clause", "/schemas", "GET", http.StatusOK, []string{"information_schema", "pg_catalog", "pg_toast", "public"}},
		{"Get schemas with custom where clause", "/schemas?schema_name=$eq.public", "GET", http.StatusOK, []string{"public"}},
		{"Get schemas with custom order clause", "/schemas?schema_name=$eq.public&_order=schema_name", "GET", http.StatusOK, []string{"public"}},
		{"Get schemas with custom order invalid clause", "/schemas?schema_name=$eq.public&_order=$eq.schema_name", "GET", http.StatusBadRequest, []string{"invalid identifier"}},
		{"Get schemas with custom where clause and pagination", "/schemas?schema_name=$eq.public&_page=1&_page_size=20", "GET", http.StatusOK, []string{"public"}},
		{"Get schemas with COUNT clause", "/schemas?_count=*", "GET", http.StatusOK, []string{`[{"count": 4}]`}},
		{"Get schemas with custom where invalid clause", "/schemas?0schema_name=$eq.public", "GET", http.StatusBadRequest, []string{"invalid identifier"}},
		{"Get schemas with noexistent column", "/schemas?schematame=$eq.test", "GET", http.StatusBadRequest, []string{"does not exist"}},
		{"Get schemas with distinct clause", "/schemas?schema_name=$eq.public&_distinct=true", "GET", http.StatusOK, []string{"public"}},
	}

	h := helpers.NewIntegrationHandlers(t)
	router := mux.NewRouter()
	router.HandleFunc("/schemas", h.Catalog.ListSchemas).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetSchemas", tc.body...)
	}
}

func TestVersionDependentGetSchemas(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        string
	}{
		{
			"Get schemas with custom where and pagination invalid",
			"/schemas?schema_name=$eq.public&_page=A",
			"GET",
			http.StatusBadRequest,
			`strconv.Atoi: parsing "A": invalid syntax`,
		},
	}

	h := helpers.NewIntegrationHandlers(t)
	router := mux.NewRouter()
	router.HandleFunc("/schemas", h.Catalog.ListSchemas).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetSchemas", tc.body)
	}
}
