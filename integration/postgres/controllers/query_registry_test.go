// nolint
package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

const (
	queriesAdminUser = "test@postgres.rest"
	queriesAdminPass = "123456"
	queriesJWTKey    = "integration-test-secret"
)

func TestQueryRegistryAuthGuards(t *testing.T) {
	base := helpers.QueriesServerURL(t)

	// List registry entries without a token.
	// Expected to fail with HTTP status Unauthorized.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry",
		nil, http.MethodGet, "", http.StatusUnauthorized, "RegistryNoToken")

	nonAdmin, err := controllers.Token(auth.User{Username: "other@postgres.rest"}, queriesJWTKey)
	if err != nil {
		t.Fatalf("sign non-admin token: %v", err)
	}

	// Non-admin JWT may not create registry entries.
	// Expected to fail with HTTP status Forbidden.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry",
		map[string]string{
			"location": "itest",
			"name":     "denied",
			"read_sql": "SELECT 1",
		},
		http.MethodPost, nonAdmin, http.StatusForbidden, "RegistryNonAdmin")
}

func TestQueryRegistryCRUD(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// List all registry entries as admin.
	// Expected to succeed with HTTP status OK and include get_all.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry",
		nil, http.MethodGet, token, http.StatusOK, "RegistryList", "get_all")

	// List registry entries filtered by location=fulltable.
	// Expected to succeed with HTTP status OK and include get_all.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry?location=fulltable",
		nil, http.MethodGet, token, http.StatusOK, "RegistryListFilter", "get_all")

	// Fetch a single registry entry by location/name.
	// Expected to succeed with HTTP status OK and include read_sql.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry/fulltable/get_all",
		nil, http.MethodGet, token, http.StatusOK, "RegistryGet", "read_sql")

	// Fetch a registry entry with an explicit database path segment.
	// Expected to succeed with HTTP status OK and include read_sql.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry/prest-test/fulltable/get_all",
		nil, http.MethodGet, token, http.StatusOK, "RegistryGetWithDB", "read_sql")

	// Fetch a registry entry that does not exist.
	// Expected to fail with HTTP status NotFound.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry/missing/nope",
		nil, http.MethodGet, token, http.StatusNotFound, "RegistryNotFound")

	// Create a new registry entry.
	// Expected to succeed with HTTP status Created.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry",
		map[string]string{
			"location": "itest",
			"name":     "sample",
			"read_sql": "SELECT 1",
		},
		http.MethodPost, token, http.StatusCreated, "RegistryCreate")

	// Update the sample entry's read_sql.
	// Expected to succeed with HTTP status OK and return SELECT 2.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry/itest/sample",
		map[string]string{"read_sql": "SELECT 2"},
		http.MethodPut, token, http.StatusOK, "RegistryUpdate", "SELECT 2")

	// Delete the sample registry entry.
	// Expected to succeed with HTTP status NoContent.
	helpers.DoAuthRequest(
		t, base+"/_QUERIES/registry/itest/sample",
		nil, http.MethodDelete, token, http.StatusNoContent, "RegistryDelete")

	// POST malformed JSON to the registry.
	// Expected to fail with HTTP status BadRequest.
	headers := map[string]string{"Authorization": "Bearer " + token}
	testutils.DoRequestRaw(
		t, base+"/_QUERIES/registry",
		[]byte(`{invalid`), http.MethodPost, http.StatusBadRequest,
		"RegistryInvalidJSON", headers)
}
