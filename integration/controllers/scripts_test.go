// nolint
package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
)

func TestExecuteScriptQuery(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Execute script GET method", "/_QUERIES/fulltable/get_all?field1=gopher", "GET", http.StatusOK},
		{"Execute script POST method", "/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira", "POST", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, "ExecuteScriptQuery")
	}
}

func TestExecuteFromScripts(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Get results using scripts and funcs by GET method", "/_QUERIES/fulltable/funcs", "GET", http.StatusOK},
		{"Get results using scripts by GET method", "/_QUERIES/fulltable/get_all?field1=gopher", "GET", http.StatusOK},
		{"Get results using scripts by GET method (2)", "/_QUERIES/fulltable/get_header", "GET", http.StatusOK},
		{"Get results using scripts by POST method", "/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira", "POST", http.StatusOK},
		{"Get results using scripts by PUT method", "/_QUERIES/fulltable/put_all?field1=trump&field2=pereira", "PUT", http.StatusOK},
		{"Get results using scripts by PATCH method", "/_QUERIES/fulltable/patch_all?field1=temer&field2=trump", "PATCH", http.StatusOK},
		{"Get results using scripts by DELETE method", "/_QUERIES/fulltable/delete_all?field1=trump", "DELETE", http.StatusOK},
		{"Get errors using nonexistent folder", "/_QUERIES/fullnon/delete_all?field1=trump", "DELETE", http.StatusBadRequest},
		{"Get errors using nonexistent script", "/_QUERIES/fulltable/some_com_all?field1=trump", "DELETE", http.StatusBadRequest},
		{"Get errors with invalid execution of sql", "/_QUERIES/fulltable/create_table?field1=test7", "POST", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, "ExecuteFromScripts")
	}
}

func TestRenderWithXML(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        string
	}{
		{"Get schemas with COUNT clause with XML Render", "/schemas?_count=*&_renderer=xml", "GET", 200, "<objects><object><count>4</count></object></objects>"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, "GetSchemas", tc.body)
	}
}

func TestSilentErrorsOnQuery(t *testing.T) {
	t.Setenv("PREST_DEBUG", "false")
	h := helpers.NewIntegrationHandlers(t)
	router := mux.NewRouter()
	router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", helpers.WithHTTPTimeout(h.Script.Execute))
	server := httptest.NewServer(router)
	defer server.Close()

	t.Log("error query silent")
	testutils.DoRequest(
		t,
		server.URL+"/_QUERIES/error/query_w_error",
		nil,
		"GET",
		http.StatusBadRequest,
		"SilentError",
		"could not execute sql, check your prest logs",
	)
}
