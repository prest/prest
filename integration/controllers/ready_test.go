package controllers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
)

func TestReadyEndpoint(t *testing.T) {
	h := helpers.NewIntegrationHandlers(t)
	r := mux.NewRouter()
	r.HandleFunc("/_ready", helpers.WithHTTPTimeout(h.Ready.Handler())).Methods("GET")
	server := httptest.NewServer(r)
	defer server.Close()

	testutils.DoRequest(t, server.URL+"/_ready", nil, "GET", http.StatusOK, "ReadyEndpoint")
}

func TestScriptRouteWithDatabase(t *testing.T) {
	h := helpers.NewIntegrationHandlers(t)
	r := mux.NewRouter()
	r.HandleFunc("/_QUERIES/{database}/{queriesLocation}/{script}", helpers.WithHTTPTimeout(h.Script.Execute))
	ts := httptest.NewServer(r)
	defer ts.Close()

	testutils.DoRequest(t, ts.URL+"/_QUERIES/prest-test/fulltable/get_all?field1=gopher", nil, "GET", http.StatusOK, "ScriptRouteWithDatabase")
}
