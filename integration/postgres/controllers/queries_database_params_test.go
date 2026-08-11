// nolint
package controllers_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/stretchr/testify/require"
)

// The queries server stores templates in the prest_queries table instead of on
// disk (testdata/prest_queries.toml, storage = "database"). Resolution takes a
// different code path — a column lookup rather than a file read — but everything
// after it, the parameter screen and the binding helpers, is shared with the
// filesystem server. None of that was covered in database mode, so a regression
// there would have gone unnoticed on the server where operators can author
// templates at runtime through the registry API.
//
// Templates are registered under the "itest" location, which prest_queries.toml
// already grants to the admin user, so no config change is needed.

// registerQuery creates a read template through the registry API.
func registerQuery(t *testing.T, base, token, name, readSQL string) {
	t.Helper()

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry",
		map[string]string{"location": "itest", "name": name, "read_sql": readSQL},
		http.MethodPost, token, http.StatusCreated, "RegisterQuery")
}

// deleteQuery removes it again so the table is left as it was found.
func deleteQuery(t *testing.T, base, token, name string) {
	t.Helper()

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry/itest/"+name,
		nil, http.MethodDelete, token, http.StatusNoContent, "DeleteQuery")
}

// authBody runs an authenticated GET and returns status and raw body, so the
// caller can assert that something is absent (DoAuthRequest only does substring
// presence checks).
func authBody(t *testing.T, url, token string) (int, string) {
	t.Helper()

	return queriesRequest(t, http.MethodGet, url, token, nil)
}

// TestQueriesDatabase_ParameterScreenApplies proves a database-stored template is
// screened exactly like a filesystem one: a single-token value carrying a SQL
// keyword survives (issue #1030), and a multi-word injection payload fails the
// request instead of composing SQL.
func TestQueriesDatabase_ParameterScreenApplies(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	registerQuery(t, base, token, "screened", `SELECT * FROM test7 WHERE name = '{{.field1}}'`)
	defer deleteQuery(t, base, token, "screened")

	// A slug containing the keyword "do" reaches the query intact.
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/screened?field1=teste-do-abc",
		nil, http.MethodGet, token, http.StatusOK, "DBScreenPreserves", "[]")

	// An interpolated injection payload is refused, naming the parameter.
	status, body := authBody(t,
		base+"/_QUERIES/itest/screened?field1="+url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
		token)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body, "field1")
	require.NotContains(t, body, "pg_shadow")
}

// TestQueriesDatabase_BindingHelpersWork proves sqlVal reaches the caller's
// unscreened value in database mode too — the migration path pREST documents for
// free-form input has to work wherever templates are stored.
func TestQueriesDatabase_BindingHelpersWork(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	registerQuery(t, base, token, "bound", `SELECT {{sqlVal "field1"}} AS echoed`)
	defer deleteQuery(t, base, token, "bound")

	// A phrase the inline screen would refuse is bound verbatim and echoed back.
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/bound?field1="+url.QueryEscape("compra do mes"),
		nil, http.MethodGet, token, http.StatusOK, "DBBoundValue", "compra do mes")

	// An injection payload is inert as a bound value: returned as data, not run.
	status, body := authBody(t,
		base+"/_QUERIES/itest/bound?field1="+url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
		token)
	require.Equal(t, http.StatusOK, status)
	require.NotContains(t, body, "rolpassword")
	require.Contains(t, body, "UNION",
		"the payload is echoed as a bound string value, which is exactly the point")
}

// TestQueriesDatabase_RawMapsUnreachable is the database-mode version of the
// containment check. This is the realistic shape of that bug: an operator with
// registry access writes the template by hand, and `{{index ._param "x"}}` looks
// like a reasonable way to reach a parameter. It must not yield the unscreened
// value.
func TestQueriesDatabase_RawMapsUnreachable(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// The template echoes whatever it can reach, so a leak is directly visible in
	// the response. An injection-shaped payload would not be: it would return
	// catalog *rows*, and asserting on the absence of the payload string would pass
	// even while the injection succeeded.
	registerQuery(t, base, token, "rawreach", `SELECT '{{index ._param "field1"}}' AS leaked`)
	defer deleteQuery(t, base, token, "rawreach")

	const marker = "unscreened-do-marker"

	status, body := authBody(t, base+"/_QUERIES/itest/rawreach?field1="+url.QueryEscape(marker), token)

	require.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, status)
	require.NotContains(t, body, marker,
		"the raw parameter map must not be reachable from a database-stored template")
}
