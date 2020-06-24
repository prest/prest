package controllers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/adapters/postgres"
	"github.com/prest/config"
	"github.com/prest/config/router"
	"github.com/prest/middlewares"
)

func TestMain(m *testing.M) {
	os.Setenv("PREST_CONF", "testdata/prest.toml")
	config.Load()

	createMockScripts(config.PrestConf.QueriesPath)
	writeMockScripts(config.PrestConf.QueriesPath)

	code := m.Run()

	removeMockScripts(config.PrestConf.QueriesPath)
	os.Exit(code)
}

func TestExecuteScriptQuery(t *testing.T) {
	r := mux.NewRouter()
	r.HandleFunc("/testing/script-get/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := ExecuteScriptQuery(r, "fulltable", "get_all")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		w.Write(resp)
	}))

	r.HandleFunc("/testing/script-post/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := ExecuteScriptQuery(r, "fulltable", "write_all")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		w.Write(resp)
	}))

	ts := httptest.NewServer(r)
	defer ts.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Execute script GET method", "/testing/script-get/?field1=gopher", "GET", http.StatusOK},
		{"Execute script POST method", "/testing/script-post/?field1=gopherzin&field2=pereira", "POST", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		doRequest(t, ts.URL+tc.url, nil, tc.method, tc.status, "ExecuteScriptQuery")
	}
}

func TestExecuteFromScripts(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", ExecuteFromScripts)
	server := httptest.NewServer(router)
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Get results using scripts and funcs by GET method", "/_QUERIES/fulltable/funcs", "GET", http.StatusOK},
		{"Get results using scripts by GET method", "/_QUERIES/fulltable/get_all?field1=gopher", "GET", http.StatusOK},
		{"Get results using scripts by POST method", "/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira", "POST", http.StatusOK},
		{"Get results using scripts by PUT method", "/_QUERIES/fulltable/put_all?field1=trump&field2=pereira", "PUT", http.StatusOK},
		{"Get results using scripts by PATCH method", "/_QUERIES/fulltable/patch_all?field1=temer&field2=trump", "PATCH", http.StatusOK},
		{"Get results using scripts by DELETE method", "/_QUERIES/fulltable/delete_all?field1=trump", "DELETE", http.StatusOK},
		// errors
		{"Get errors using nonexistent folder", "/_QUERIES/fullnon/delete_all?field1=trump", "DELETE", http.StatusBadRequest},
		{"Get errors using nonexistent script", "/_QUERIES/fulltable/some_com_all?field1=trump", "DELETE", http.StatusBadRequest},
		{"Get errors with invalid execution of sql", "/_QUERIES/fulltable/create_table?field1=test7", "POST", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		doRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "ExecuteFromScripts")
	}
}

func TestRenderWithXML(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        string
	}{
		{"Get schemas with COUNT clause with XML Render", "/schemas?_count=*&_renderer=xml", "GET", 200, "<objects><object><count>6</count></object></objects>"},
	}
	os.Setenv("PREST_DEBUG", "true")
	config.Load()
	postgres.Load()
	n := middlewares.GetApp()
	r := router.Get()

	r.HandleFunc("/schemas", GetSchemas).Methods("GET")
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		doRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetSchemas", tc.body)

	}
}
