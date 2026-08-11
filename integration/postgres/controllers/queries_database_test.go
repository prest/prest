// nolint
package controllers_test

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/stretchr/testify/require"
)

// The queries server runs with storage = "database": every template under
// testdata/queries is imported into prest_queries at startup and resolved from a
// column, not from disk. Everything after resolution — the parameter screen, the
// header screen, the binding helpers, the template function registry — is shared
// with the filesystem server, but only three of those shapes were covered here.
//
// The tests below execute the imported copy of every template shape that
// testdata/queries contains, mirroring the filesystem-mode assertions in
// integration/suites/controllers/scripts_test.go so a divergence between the two
// servers shows up as a failure rather than as silence. Two cases have no
// filesystem counterpart and only exist here: a verb column that is empty, and
// templates authored at runtime through the registry API.
//
// Grants are per location+name in testdata/prest_queries.toml (restrict = true).

// queriesRequest runs an authenticated request against the queries server and
// returns status and raw body. helpers.DoAuthRequest can only assert that a body
// CONTAINS a string and takes no header map; several templates below need both
// the negative assertion and a custom header.
func queriesRequest(t *testing.T, method, url, token string, headers map[string]string) (int, string) {
	t.Helper()

	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	req.Header.Set("X-Application", "prest")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// registerQueryFull creates a template carrying any subset of the four verb
// columns; registerQuery only sends read_sql.
func registerQueryFull(t *testing.T, base, token, name string, verbs map[string]string) {
	t.Helper()

	payload := map[string]string{"location": "itest", "name": name}
	for column, sql := range verbs {
		payload[column] = sql
	}

	helpers.DoAuthRequest(t, base+"/_QUERIES/registry", payload,
		http.MethodPost, token, http.StatusCreated, "RegisterQueryFull")
}

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

// TestQueriesDatabase_HeaderHelpers covers the two templates that interpolate a
// header inline. The credential case is stronger on this server than on the
// filesystem one: every request here carries a real JWT, so the blanking of
// Authorization is exercised with an actual secret rather than a fixture string.
func TestQueriesDatabase_HeaderHelpers(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// get_header is `SELECT '{{index .header "X-Application"}}'`. A quote breakout
	// in the header must be screened to an empty literal, not composed into SQL.
	status, body := queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/get_header", token,
		map[string]string{"X-Application": "' UNION SELECT rolpassword::text FROM pg_authid --"})
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `""`, "the screened header renders as an empty literal")
	require.NotContains(t, body, "rolpassword")

	// A benign header value still reaches the query untouched.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_header",
		nil, http.MethodGet, token, http.StatusOK, "DBHeaderInline", `"prest"`)

	// get_auth_header is the same shape reading Authorization. The caller's token
	// must never be interpolated into the SQL text (it is logged at debug level).
	status, body = queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/get_auth_header", token, nil)
	require.Equal(t, http.StatusOK, status)
	require.NotContains(t, body, token, "a bearer token must never reach the SQL text")
}

// TestQueriesDatabase_BoundHeaderValues covers `{{sqlVal "header.X"}}`, which
// binds the UNSCREENED header value. Binding must not become a way around the
// credential blanking, which is about secrecy rather than SQL composition.
func TestQueriesDatabase_BoundHeaderValues(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// A phrase the inline screen would blank is bound verbatim and echoed back.
	status, body := queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/get_bound_header", token,
		map[string]string{"X-Application": "compra do mes"})
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "compra do mes")

	// The credential header binds empty even though a live token was sent.
	status, body = queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/get_bound_auth_header", token, nil)
	require.Equal(t, http.StatusOK, status)
	require.NotContains(t, body, token, "a bearer token must never be bindable into a query")
}

// TestQueriesDatabase_BoundPlaceholderOrdering checks that two bound values arrive
// as $1 and $2 in the order the template names them, not in query-string order.
// Wrong pairing would silently answer with another record's rows.
func TestQueriesDatabase_BoundPlaceholderOrdering(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// get_bound_two: WHERE name = {{sqlVal "field1"}} AND surname = {{sqlVal "field2"}}.
	// The seeded row is ('gopher', 'da silva'); matching it proves the pairing.
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/get_bound_two?field1=gopher&field2="+url.QueryEscape("da silva"),
		nil, http.MethodGet, token, http.StatusOK, "DBBoundOrdering", "da silva")

	// Swapping the values must match nothing — if the order were wrong, this is the
	// request that would succeed.
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/get_bound_two?field1="+url.QueryEscape("da silva")+"&field2=gopher",
		nil, http.MethodGet, token, http.StatusOK, "DBBoundOrderingSwapped", "[]")
}

// TestQueriesDatabase_ListHelpers covers the two IN-list helpers, which diverge on
// purpose: inFormat interpolates, so a rejected element fails the request rather
// than shrinking the list; sqlList binds, so the same payload is inert data.
func TestQueriesDatabase_ListHelpers(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// All elements survive the screen: inFormat joins them into a quoted IN list.
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/get_all_slice?field1=nobody&field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "DBSliceAllSafe", "gopher")

	// One rejected element fails the whole request instead of querying a truncated list.
	status, body := queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/get_all_slice?field1=gopher&field1="+
			url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"), token, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.NotContains(t, body, "pg_shadow")

	// sqlList binds every element, so the seeded row still matches through the
	// second value and the payload is data rather than syntax.
	status, body = queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/get_bound_list?field1=gopher&field1="+
			url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"), token, nil)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "gopher")
	require.NotContains(t, body, "passwd")

	// Single value: sqlList takes the non-slice branch and still renders ($1).
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/get_bound_list?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "DBBoundListSingle", "gopher")
}

// TestQueriesDatabase_IdentHelper covers `{{ident "field1"}}`. An identifier cannot
// be bound, so ident validates it against a strict charset and double-quotes it;
// a rejected one must fail without echoing the caller's value back.
func TestQueriesDatabase_IdentHelper(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// A valid table name resolves and the query runs.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_ident?field1=test7",
		nil, http.MethodGet, token, http.StatusOK, "DBIdent", "gopher")

	var rejected = []struct {
		description string
		value       string
	}{
		{"quote breakout in an identifier", `test7"; DROP TABLE test7; --`},
		{"identifier with a hyphen", "foo-bar"},
		{"identifier starting with a digit", "1abc"},
		{"empty identifier", ""},
		{"whitespace composition", "test7 UNION SELECT 1"},
	}
	for _, tc := range rejected {
		t.Log("ident rejects: " + tc.description)
		status, body := queriesRequest(t, http.MethodGet,
			base+"/_QUERIES/fulltable/get_ident?field1="+url.QueryEscape(tc.value), token, nil)
		require.Equal(t, http.StatusBadRequest, status, tc.description)

		// The template error names the template internals and the offending value;
		// only the safe summary may reach the caller.
		require.NotContains(t, body, "DROP TABLE", tc.description)
		require.NotContains(t, body, "invalid identifier", tc.description)
		require.Contains(t, body, "check your prest logs", tc.description)
	}

	// The DDL in the payload above must never have executed: test7 still answers.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_ident?field1=test7",
		nil, http.MethodGet, token, http.StatusOK, "DBIdentTableIntact", "gopher")
}

// TestQueriesDatabase_LimitOffset covers both limitOffset fixtures: the one taking
// caller-supplied pagination and the one with literal arguments. The bad-pagination
// case pins a sharp edge rather than a fix — the clause vanishes instead of erroring.
func TestQueriesDatabase_LimitOffset(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// Valid pagination: the clause renders and the query succeeds.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_limitoffset_params?page=1&size=1",
		nil, http.MethodGet, token, http.StatusOK, "DBLimitOffsetValid", `"name"`)

	// Literal arguments: no caller input involved, so it always renders.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/limitoffset",
		nil, http.MethodGet, token, http.StatusOK, "DBLimitOffsetLiteral", `"name"`)

	// Non-numeric page: the clause is dropped rather than erroring, and every row
	// comes back. Documented-but-undesirable; pinned so it cannot change silently.
	status, body := queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/get_limitoffset_params?page=abc&size=1", token, nil)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `"name"`,
		"limitOffset drops the clause on a parse error, returning unpaginated rows")
}

// TestQueriesDatabase_DefaultOrValue covers `defaultOrValue`, whose interaction with
// a refused parameter is easy to get wrong: the default applies only when the key is
// absent, and a rejected value still occupies the key, so the request must fail
// rather than quietly answer with the default's rows.
func TestQueriesDatabase_DefaultOrValue(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// No parameter at all: the default "gopher" applies and the seeded row returns.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/funcs",
		nil, http.MethodGet, token, http.StatusOK, "DBDefaultApplied", "gopher")

	// A value that survives the screen overrides the default.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/funcs?field1=teste-do-abc",
		nil, http.MethodGet, token, http.StatusOK, "DBDefaultOverridden", "[]")

	// A rejected value fails instead of falling through to the default.
	status, body := queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/funcs?field1="+
			url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"), token, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.NotContains(t, body, "gopher",
		"a refused parameter must not fall through to the template default")
}

// TestQueriesDatabase_UnEscape is adversarial: unEscape URL-decodes its argument, so
// a surviving percent-escape would let a template decode `%27` back into the quote
// the screen exists to remove. `%` is itself outside the allow-list, so the payload
// is refused one level earlier — load-bearing and undocumented, hence pinned.
func TestQueriesDatabase_UnEscape(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// Plain value: unEscape is a no-op on a screened value.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_unescape?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "DBUnEscapePlain", "gopher")

	var payloads = []struct {
		description string
		value       string
	}{
		{"single-encoded quote breakout", "%27 OR 1=1 --"},
		{"double-encoded quote breakout", "%2527%20OR%201%3D1"},
		{"double-encoded UNION arm", "%25%32%37 UNION SELECT passwd FROM pg_shadow"},
	}
	for _, tc := range payloads {
		t.Log("unEscape cannot reconstruct SQL syntax: " + tc.description)
		status, body := queriesRequest(t, http.MethodGet,
			base+"/_QUERIES/fulltable/get_unescape?field1="+url.QueryEscape(tc.value), token, nil)
		require.Equal(t, http.StatusBadRequest, status, tc.description)
		require.NotContains(t, body, "pg_shadow", tc.description)
	}
}

// TestQueriesDatabase_UnquotedContextInjection covers `WHERE 1 = {{.field1}}`, where
// letters, digits and spaces alone compose SQL because there is no surrounding
// quote to break out of. The keyword screen must refuse those payloads.
func TestQueriesDatabase_UnquotedContextInjection(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// Baseline: a plain numeric value still reaches the query and succeeds.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_unquoted?field1=1",
		nil, http.MethodGet, token, http.StatusOK, "DBUnquotedBenign")

	var payloads = []struct {
		description string
		value       string
	}{
		{"UNION arm dumping a whole row via ::text", "1 UNION SELECT test7::text FROM test7"},
		{"catalog read of role password hashes", "1 UNION SELECT passwd FROM pg_shadow"},
		{"row-filter bypass needing no UNION", "1 OR true"},
	}
	for _, tc := range payloads {
		t.Log("unquoted context refuses: " + tc.description)
		status, body := queriesRequest(t, http.MethodGet,
			base+"/_QUERIES/fulltable/get_unquoted?field1="+url.QueryEscape(tc.value), token, nil)
		require.Equal(t, http.StatusBadRequest, status, tc.description)
		require.NotContains(t, body, "pg_shadow", tc.description)
	}
}

// TestQueriesDatabase_RawMapsUnreachableFromImportedTemplates completes the
// containment check for the two fixtures the runtime-registered case in
// queries_database_params_test.go does not use: rendering the whole raw parameter
// map, and indexing the raw header map. Either outcome is acceptable — a 400
// because the template failed to render, or a 200 whose body simply lacks the
// value — but the unscreened input must never appear.
func TestQueriesDatabase_RawMapsUnreachableFromImportedTemplates(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	var fixtures = []struct {
		description string
		script      string
	}{
		{"render the whole raw param map", "get_raw_param_dot"},
		{"index into the raw param map", "get_raw_param_index"},
		{"index into the raw header map", "get_raw_header_index"},
	}

	const payload = "1 UNION SELECT passwd FROM pg_shadow"

	for _, f := range fixtures {
		t.Log(f.description + " must not reach the caller's unscreened value")
		status, body := queriesRequest(t, http.MethodGet,
			base+"/_QUERIES/fulltable/"+f.script+"?field1="+url.QueryEscape(payload), token,
			map[string]string{"X-Application": payload})

		require.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, status, f.description)
		require.NotContains(t, body, "pg_shadow", f.description)
		require.NotContains(t, body, "UNION", f.description)
		require.NotContains(t, body, "passwd", f.description)
	}
}

// TestQueriesDatabase_BoundWriteVerbs exercises POST, PUT/PATCH and DELETE against
// the bound templates. In database mode each verb reads a different column of
// prest_queries, so this is the only test that proves the verb→column mapping is
// right for anything other than GET.
//
// test7 is shared with the filesystem suite, so the row uses a marker no other
// test touches and is deleted at the end. The values carry a keyword and spaces,
// which means they would be refused if the templates interpolated them.
func TestQueriesDatabase_BoundWriteVerbs(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	const (
		name    = "dbmode do write"
		surname = "sobrenome do teste"
		updated = "atualizado do teste"
	)

	// INSERT through bound parameters (write_sql column). rows_affected proves the
	// statement ran with the real values rather than blanks.
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/write_bound?field1="+url.QueryEscape(name)+
			"&field2="+url.QueryEscape(surname),
		nil, http.MethodPost, token, http.StatusOK, "DBBoundInsert", `"rows_affected":1`)

	// Read it back: the row must carry the exact values, so nothing was blanked.
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1="+url.QueryEscape(name),
		nil, http.MethodGet, token, http.StatusOK, "DBBoundInsertReadback", surname)

	// UPDATE through PUT (update_sql column).
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/update_bound?field1="+url.QueryEscape(name)+
			"&field2="+url.QueryEscape(updated),
		nil, http.MethodPut, token, http.StatusOK, "DBBoundUpdate", `"rows_affected":1`)
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1="+url.QueryEscape(name),
		nil, http.MethodGet, token, http.StatusOK, "DBBoundUpdateReadback", updated)

	// PATCH resolves the same column and must behave identically.
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/update_bound?field1="+url.QueryEscape(name)+
			"&field2="+url.QueryEscape(surname),
		nil, http.MethodPatch, token, http.StatusOK, "DBBoundPatch", `"rows_affected":1`)

	// DELETE through a bound parameter (delete_sql column), then confirm it is gone.
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/delete_bound?field1="+url.QueryEscape(name),
		nil, http.MethodDelete, token, http.StatusOK, "DBBoundDelete", `"rows_affected":1`)
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1="+url.QueryEscape(name),
		nil, http.MethodGet, token, http.StatusOK, "DBBoundDeleteReadback", "[]")
}

// TestQueriesDatabase_InterpolatedWriteVerbs covers the write_all / put_all /
// patch_all / delete_all shapes — values interpolated into the statement text —
// on a template authored through the registry API, which is how an operator
// creates one on this server. The on-disk fulltable copies stay ungranted because
// queries_acl_test.go asserts that POST fulltable/write_all is refused.
func TestQueriesDatabase_InterpolatedWriteVerbs(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	registerQueryFull(t, base, token, "wverbs", map[string]string{
		"read_sql":   `SELECT * FROM test7 WHERE name = '{{.field1}}'`,
		"write_sql":  `INSERT INTO test7 (name, surname) VALUES ('{{.field1}}', '{{.field2}}')`,
		"update_sql": `UPDATE test7 SET surname = '{{.field2}}' WHERE name = '{{.field1}}'`,
		"delete_sql": `DELETE FROM test7 WHERE name = '{{.field1}}'`,
	})
	defer deleteQuery(t, base, token, "wverbs")

	// Single-token values pass the screen, so the row is written with real values.
	const marker = "dbmode-interp-marker"
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/itest/wverbs?field1="+marker+"&field2=sobrenome-um",
		nil, http.MethodPost, token, http.StatusOK, "DBInterpInsert", `"rows_affected":1`)
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/wverbs?field1="+marker,
		nil, http.MethodGet, token, http.StatusOK, "DBInterpReadback", "sobrenome-um")

	// PUT and PATCH both resolve update_sql.
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/itest/wverbs?field1="+marker+"&field2=sobrenome-dois",
		nil, http.MethodPut, token, http.StatusOK, "DBInterpUpdate", `"rows_affected":1`)
	helpers.DoAuthRequest(t,
		base+"/_QUERIES/itest/wverbs?field1="+marker+"&field2=sobrenome-tres",
		nil, http.MethodPatch, token, http.StatusOK, "DBInterpPatch", `"rows_affected":1`)
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/wverbs?field1="+marker,
		nil, http.MethodGet, token, http.StatusOK, "DBInterpPatchReadback", "sobrenome-tres")

	// A payload aimed at the mutating statement is refused before it composes.
	status, body := queriesRequest(t, http.MethodPost,
		base+"/_QUERIES/itest/wverbs?field1="+
			url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow")+"&field2=x", token, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body, "field1", "the error must name the offending parameter")
	require.NotContains(t, body, "pg_shadow", "the error must not echo the value")

	// DELETE resolves delete_sql; the marker row is removed and the readback is empty.
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/wverbs?field1="+marker,
		nil, http.MethodDelete, token, http.StatusOK, "DBInterpDelete", `"rows_affected":1`)
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/wverbs?field1="+marker,
		nil, http.MethodGet, token, http.StatusOK, "DBInterpDeleteReadback", "[]")
}

// TestQueriesDatabase_MissingVerbTemplate has no filesystem counterpart: on disk a
// missing verb means a missing file, while here the row exists and only the column
// for the requested verb is empty. That must fail the request rather than execute
// an empty statement or fall back to another verb's SQL.
func TestQueriesDatabase_MissingVerbTemplate(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// read_sql only; the grant carries "write" so the request reaches the resolver
	// instead of being refused by the ACL, which is what makes this a resolver test.
	registerQuery(t, base, token, "readonly", `SELECT 1 AS ok`)
	defer deleteQuery(t, base, token, "readonly")

	// GET works: the column it needs is populated.
	helpers.DoAuthRequest(t, base+"/_QUERIES/itest/readonly",
		nil, http.MethodGet, token, http.StatusOK, "DBReadOnlyGet", "ok")

	// POST asks for write_sql, which is empty.
	status, body := queriesRequest(t, http.MethodPost, base+"/_QUERIES/itest/readonly", token, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body, "readonly", "the error must name the script that could not be loaded")
}

// TestQueriesDatabase_TemplateAndSQLErrors covers the failure fixtures: a template
// that does not parse, a statement the database rejects, and the DDL shape of
// create_table.write.sql. All three must surface as a 400 carrying only the safe
// summary — never template internals or driver detail.
func TestQueriesDatabase_TemplateAndSQLErrors(t *testing.T) {
	base := helpers.QueriesServerURL(t)
	token := helpers.LoginToken(t, base, queriesAdminUser, queriesAdminPass)

	// parse_syntax_invalid has an unterminated action, so rendering fails.
	status, body := queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/fulltable/parse_syntax_invalid?field1=gopher", token, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body, "check your prest logs")
	require.NotContains(t, body, "unclosed action", "template internals stay in the logs")

	// error/query_w_error selects from a table that does not exist: the statement
	// renders fine and fails in the database.
	status, body = queriesRequest(t, http.MethodGet,
		base+"/_QUERIES/error/query_w_error", token, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body, "check your prest logs")
	require.NotContains(t, body, "non_existing_table", "driver detail stays in the logs")

	// The create_table shape: DDL with an interpolated identifier. Targeting the
	// existing test7 makes the database refuse it, so the check is non-destructive.
	registerQueryFull(t, base, token, "ddl", map[string]string{
		"write_sql": `CREATE TABLE {{.field1}};`,
	})
	defer deleteQuery(t, base, token, "ddl")

	status, body = queriesRequest(t, http.MethodPost,
		base+"/_QUERIES/itest/ddl?field1=test7", token, nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body, "check your prest logs")

	// test7 is intact — the failed DDL neither dropped nor replaced it.
	helpers.DoAuthRequest(t, base+"/_QUERIES/fulltable/get_all?field1=gopher",
		nil, http.MethodGet, token, http.StatusOK, "DBTableIntactAfterDDL", "gopher")
}
