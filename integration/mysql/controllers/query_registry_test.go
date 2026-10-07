package mysql_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
)

func TestMySQLQueryRegistryLifecycle(t *testing.T) {
	// The MySQL prest_queries table (ON DUPLICATE KEY upsert, ? binds) runs the
	// same lifecycle as Postgres.
	helpers.RunQueryRegistryLifecycle(t, helpers.QueryRegistryTarget{
		BaseURL:   helpers.QueriesServerURL(t),
		Database:  "shop",
		AdminUser: "test@postgres.rest",
		AdminPass: "123456",
		JWTKey:    "integration-test-secret-key-32b!!",
	})
}

func TestMySQLQueryRegistryImportedBoundTemplate(t *testing.T) {
	// A template imported from testdata/queries binds sqlVal as ? on MySQL.
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")
	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/fulltable/get_bound", nil, http.MethodGet, token, http.StatusOK, "imported", "sqlVal")
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_bound?field1=gopher", nil, http.MethodGet, token, http.StatusOK, "execute", "da silva")
}
