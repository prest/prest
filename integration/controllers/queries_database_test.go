// nolint
package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
)

func TestQueriesDatabaseExecution(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// Test the fulltable/get_all endpoint
	// Expected to succeed and return the body in the expectedBody slice.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/fulltable/get_all?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "QueriesDBExecute")

	// Test the fulltable/get_all endpoint with a database name
	// Expected to succeed and return the body in the expectedBody slice.
	// It will use the database name from the URL.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/prest-test/fulltable/get_all?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "QueriesDBExecuteWithDB")

	// Test the registry endpoint
	// Expected to succeed and return the body in the expectedBody slice.
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", map[string]string{
		"location": "itest",
		"name":     "ephemeral",
		"read_sql": "SELECT 1",
	}, http.MethodPost, token, http.StatusCreated, "QueriesDBCreateEphemeral")

	// Test the registry endpoint with a database name
	// Expected to succeed and return the body in the expectedBody slice.
	// It will use the database name from the URL.
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/itest/ephemeral",
		nil, http.MethodDelete, token, http.StatusNoContent, "QueriesDBDeleteEphemeral")

	// Test the registry endpoint with a database name
	// Expected to fail and return the body in the expectedBody slice.
	// It will use the database name from the URL.
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/ephemeral",
		nil, http.MethodGet, token, http.StatusBadRequest, "QueriesDBMissingAfterDelete")
}
