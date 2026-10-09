// nolint
package controllers_test

import (
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
)

func TestQueryRegistryLifecycle(t *testing.T) {
	// Shared registry lifecycle on the Postgres queries server (prest_queries.toml):
	// verb execution, live PUT, alias routing and error paths.
	helpers.RunQueryRegistryLifecycle(t, helpers.QueryRegistryTarget{
		BaseURL:   helpers.QueriesServerURL(t),
		Database:  "prest-test",
		AdminUser: queriesAdminUser,
		AdminPass: queriesAdminPass,
		JWTKey:    queriesJWTKey,
	})
}
