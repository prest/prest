package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestReadyEndpoint(t *testing.T) {
	base := helpers.ServerURL(t)

	// Probe the readiness endpoint on the default prestd.
	// Expected to succeed with HTTP status OK when dependencies are ready.
	testutils.DoRequest(
		t, base+"/_ready",
		nil, "GET", http.StatusOK, "ReadyEndpoint")
}

func TestScriptRouteWithDatabase(t *testing.T) {
	base := helpers.ServerURL(t)

	// Execute a custom query with an explicit database path segment.
	// Expected to succeed with HTTP status OK.
	// The database name in the URL selects which DB runs the script.
	testutils.DoRequest(
		t, base+"/_QUERIES/prest-test/fulltable/get_all?field1=gopher",
		nil, "GET", http.StatusOK, "ScriptRouteWithDatabase")
}
