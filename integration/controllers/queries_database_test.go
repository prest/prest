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

	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_all?field1=gopher", nil, http.MethodGet, token, http.StatusOK, "QueriesDBExecute")
	helpers.DoAuthRequest(t, base+"/_QUERIES/prest-test/fulltable/get_all?field1=gopher", nil, http.MethodGet, token, http.StatusOK, "QueriesDBExecuteWithDB")

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", map[string]string{
		"location": "itest",
		"name":     "ephemeral",
		"read_sql": "SELECT 1",
	}, http.MethodPost, token, http.StatusCreated, "QueriesDBCreateEphemeral")
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/itest/ephemeral", nil, http.MethodDelete, token, http.StatusNoContent, "QueriesDBDeleteEphemeral")
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/ephemeral", nil, http.MethodGet, token, http.StatusBadRequest, "QueriesDBMissingAfterDelete")
}
