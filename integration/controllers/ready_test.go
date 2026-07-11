package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestReadyEndpoint(t *testing.T) {
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/_ready", nil, "GET", http.StatusOK, "ReadyEndpoint")
}

func TestQueriesServerReady(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	testutils.DoRequest(t, base+"/_ready", nil, "GET", http.StatusOK, "QueriesServerReady")
}

func TestScriptRouteWithDatabase(t *testing.T) {
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/_QUERIES/prest-test/fulltable/get_all?field1=gopher", nil, "GET", http.StatusOK, "ScriptRouteWithDatabase")
}
