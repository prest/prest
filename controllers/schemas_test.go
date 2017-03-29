package controllers

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/api"
)

func TestGetSchemas(t *testing.T) {

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        string
	}{
		{"Get schemas without custom where clause", "/schemas", "GET", 200, "[{\"schema_name\":\"information_schema\"},{\"schema_name\":\"pg_catalog\"},{\"schema_name\":\"pg_temp_1\"},{\"schema_name\":\"pg_toast\"},{\"schema_name\":\"pg_toast_temp_1\"},{\"schema_name\":\"public\"}]"},
		{"Get schemas with custom where clause", "/schemas?schema_name=$eq.public", "GET", 200, "[{\"schema_name\":\"public\"}]"},
		{"Get schemas with custom order clause", "/schemas?schema_name=$eq.public&_order=schema_name", "GET", 200, "[{\"schema_name\":\"public\"}]"},
		{"Get schemas with custom where clause and pagination", "/schemas?schema_name=$eq.public&_page=1&_page_size=20", "GET", 200, "[{\"schema_name\":\"public\"}]"},
		{"Get schemas with COUNT clause", "/schemas?_count=*", "GET", 200, "[{\"count\":6}]"},
		{"Get schemas with COUNT clause and xml", "/schemas?_count=*&_renderer=xml", "GET", 200, ""},
		{"Get schemas with custom where invalid clause", "/schemas?0schema_name=$eq.public", "GET", 400, "invalid identifier: 0schema_name\n"},
		{"Get schemas with custom where and pagination invalid", "/schemas?schema_name=$eq.public&_page=A", "GET", 400, "Paging error\n"},
		{"Get schemas with noexistent column", "/schemas?schematame=$eq.test", "GET", 500, "pq: column \"schematame\" does not exist\n"},
	}

	router := mux.NewRouter()
	router.HandleFunc("/schemas", GetSchemas).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	r := api.Request{}
	for _, tc := range testCases {
		t.Log(tc.description)
		doRequest(t, server.URL+tc.url, r, tc.method, tc.status, "GetSchemas", tc.body)

	}
}
