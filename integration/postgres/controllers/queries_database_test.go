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

	// Test the fulltable/get_all custom query.
	// Expected to succeed with HTTP status OK.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/fulltable/get_all?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "QueriesDBExecute")

	// Test get_all with an explicit database name in the path.
	// Expected to succeed with HTTP status OK.
	// The database segment selects which registered DB runs the query.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/prest-test/fulltable/get_all?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "QueriesDBExecuteWithDB")

	// Register an ephemeral custom query via the registry API.
	// Expected to succeed with HTTP status Created.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry",
		map[string]string{
			"location": "itest",
			"name":     "ephemeral",
			"read_sql": "SELECT 1",
		},
		http.MethodPost, token, http.StatusCreated, "QueriesDBCreateEphemeral")

	// Delete the ephemeral registry entry.
	// Expected to succeed with HTTP status NoContent.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry/itest/ephemeral",
		nil, http.MethodDelete, token, http.StatusNoContent, "QueriesDBDeleteEphemeral")

	// Execute the deleted query path after registry removal.
	// Expected to fail with HTTP status BadRequest because the script is gone.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/itest/ephemeral",
		nil, http.MethodGet, token, http.StatusBadRequest, "QueriesDBMissingAfterDelete")
}
