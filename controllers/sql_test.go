package controllers

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/config"
)

func TestMain(m *testing.M) {
	config.InitConf()
	createMockScripts(config.PREST_CONF.QueriesPath)
	writeMockScripts(config.PREST_CONF.QueriesPath)

	code := m.Run()

	removeMockScripts(config.PREST_CONF.QueriesPath)
	os.Exit(code)
}

func TestExecuteFromScripts(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", ExecuteFromScripts)
	server := httptest.NewServer(router)
	defer server.Close()

	r := api.Request{}

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Get results using scripts by GET method", "/_QUERIES/fulltable/get_all?field1=gopher", "GET", 200},
		{"Get results using scripts by POST method", "/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira", "POST", 200},
		{"Get results using scripts by PUT method", "/_QUERIES/fulltable/put_all?field1=trump&field2=pereira", "PUT", 200},
		{"Get results using scripts by PATCH method", "/_QUERIES/fulltable/patch_all?field1=temer&field2=trump", "PATCH", 200},
		{"Get results using scripts by DELETE method", "/_QUERIES/fulltable/delete_all?field1=trump", "DELETE", 200},
		// errors
		{"Get errors using nonexistent folder", "/_QUERIES/fullnon/delete_all?field1=trump", "DELETE", 400},
		{"Get errors using nonexistent script", "/_QUERIES/fulltable/some_com_all?field1=trump", "DELETE", 400},
		{"Get errors with invalid params in script", "/_QUERIES/fulltable/get_all?column1=gopher", "GET", 400},
		{"Get errors with invalid execution of sql", "/_QUERIES/fulltable/create_table?field1=test7", "POST", 400},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		doRequest(t, server.URL+tc.url, r, tc.method, tc.status, "ExecuteFromScripts")
	}
}
