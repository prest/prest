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

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", nil, http.MethodGet, "", http.StatusUnauthorized, "RegistryNoToken")

	nonAdmin, err := controllers.Token(auth.User{Username: "other@postgres.rest"}, queriesJWTKey)
	if err != nil {
		t.Fatalf("sign non-admin token: %v", err)
	}
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", map[string]string{
		"location": "itest",
		"name":     "denied",
		"read_sql": "SELECT 1",
	}, http.MethodPost, nonAdmin, http.StatusForbidden, "RegistryNonAdmin")
}

func TestQueryRegistryCRUD(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", nil, http.MethodGet, token, http.StatusOK, "RegistryList", "get_all")
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry?location=fulltable", nil, http.MethodGet, token, http.StatusOK, "RegistryListFilter", "get_all")
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/fulltable/get_all", nil, http.MethodGet, token, http.StatusOK, "RegistryGet", "read_sql")
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/prest-test/fulltable/get_all", nil, http.MethodGet, token, http.StatusOK, "RegistryGetWithDB", "read_sql")
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/missing/nope", nil, http.MethodGet, token, http.StatusNotFound, "RegistryNotFound")

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", map[string]string{
		"location": "itest",
		"name":     "sample",
		"read_sql": "SELECT 1",
	}, http.MethodPost, token, http.StatusCreated, "RegistryCreate")

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/itest/sample", map[string]string{
		"read_sql": "SELECT 2",
	}, http.MethodPut, token, http.StatusOK, "RegistryUpdate", "SELECT 2")

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/itest/sample", nil, http.MethodDelete, token, http.StatusNoContent, "RegistryDelete")

	headers := map[string]string{"Authorization": "Bearer " + token}
	testutils.DoRequestRaw(t, base+"/_QUERIES/registry", []byte(`{invalid`), http.MethodPost, http.StatusBadRequest, "RegistryInvalidJSON", headers)
}
