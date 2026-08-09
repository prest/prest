// nolint
package controllers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

func TestExecuteScriptQuery(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"GET get_all script returns OK", "/_QUERIES/fulltable/get_all?field1=gopher", "GET", http.StatusOK},
		{"POST write_all script returns OK", "/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira", "POST", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, "ExecuteScriptQuery")
	}
}

func TestExecuteFromScripts(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"GET funcs script returns OK", "/_QUERIES/fulltable/funcs", "GET", http.StatusOK},
		{"GET get_all script returns OK", "/_QUERIES/fulltable/get_all?field1=gopher", "GET", http.StatusOK},
		{"GET get_header script returns OK", "/_QUERIES/fulltable/get_header", "GET", http.StatusOK},
		{"POST write_all script returns OK", "/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira", "POST", http.StatusOK},
		{"PUT put_all script returns OK", "/_QUERIES/fulltable/put_all?field1=trump&field2=pereira", "PUT", http.StatusOK},
		{"PATCH patch_all script returns OK", "/_QUERIES/fulltable/patch_all?field1=temer&field2=trump", "PATCH", http.StatusOK},
		{"DELETE delete_all script returns OK", "/_QUERIES/fulltable/delete_all?field1=trump", "DELETE", http.StatusOK},
		{"DELETE nonexistent folder returns BadRequest", "/_QUERIES/fullnon/delete_all?field1=trump", "DELETE", http.StatusBadRequest},
		{"DELETE nonexistent script returns BadRequest", "/_QUERIES/fulltable/some_com_all?field1=trump", "DELETE", http.StatusBadRequest},
		{"POST invalid SQL script returns BadRequest", "/_QUERIES/fulltable/create_table?field1=test7", "POST", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, "ExecuteFromScripts")
	}
}

// TestExecuteFromScripts_RejectsHeaderInjection guards the unauthenticated SQL
// injection where get_header.read.sql interpolates the X-Application header
// directly into `SELECT '{{index .header "X-Application"}}'` with no
// sanitization, unlike query parameters which already go through
// sanitizeScriptParam. A header carrying a quote breakout used to let an
// unauthenticated request UNION in arbitrary rows (e.g. pg_authid password
// hashes). extractHeaders now routes header values through the same
// sanitizeScriptParam gate as query parameters, so the payload is neutralized
// to an empty string instead of breaking out of the SQL literal.
func TestExecuteFromScripts_RejectsHeaderInjection(t *testing.T) {
	base := helpers.ServerURL(t)

	headers := map[string]string{
		"X-Application": "' UNION SELECT rolpassword::text FROM pg_authid WHERE rolname='postgres' -- ",
	}
	testutils.DoRequestWithHeaders(
		t, base+"/_QUERIES/fulltable/get_header", nil, "GET", http.StatusOK,
		"ExecuteFromScripts", headers, `[{"?column?": ""}]`,
	)
}

// TestExecuteFromScripts_RejectsPathTraversal guards the path-injection alert
// (CodeQL go/path-injection): the queries location and script name are joined onto
// the configured queries directory to locate a .sql file on disk. A traversing
// segment must never resolve to a file outside that directory — the request is
// rejected, not served.
func TestExecuteFromScripts_RejectsPathTraversal(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		status      int
	}{
		// Backslash traversal: not a URL path separator, so unlike "../.." this
		// reaches the handler as a literal segment instead of being path-cleaned
		// into a redirect by the router. IsSafeSegment rejects it.
		{
			"GET script with backslash traversal in the location is rejected",
			"/_QUERIES/" + url.PathEscape(`..\..`) + "/get_all?field1=gopher",
			http.StatusBadRequest,
		},
		{
			"GET script with backslash traversal in the name is rejected",
			"/_QUERIES/fulltable/" + url.PathEscape(`..\..\get_all`),
			http.StatusBadRequest,
		},
		// A dot is outside IsSafeSegment's allow-list, so a caller cannot name the
		// on-disk file directly (get_all.read.sql) to sidestep the verb suffixing.
		{
			"GET script with a dotted segment is rejected",
			"/_QUERIES/fulltable/get_all.read",
			http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, "GET", tc.status, "ExecuteFromScripts")
	}
}

// TestExecuteFromScripts_DropsCredentialHeaders guards the credential leak where
// a script template referencing a credential header interpolated the caller's
// bearer token into the SQL text, which the adapter then logged at debug level
// (CodeQL go/clear-text-logging, adapters/postgres/postgres.go QueryCtx).
// sanitizeScriptParam does not help here: a JWT is plain base64url and passes its
// allow-list untouched. extractHeaders now blanks credential headers, so the
// template renders an empty literal and the token never reaches the SQL string.
func TestExecuteFromScripts_DropsCredentialHeaders(t *testing.T) {
	base := helpers.ServerURL(t)

	token := "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJnb3BoZXIifQ.c2ln"

	// get_auth_header.read.sql is `SELECT '{{index .header "Authorization"}}'`.
	// The request still succeeds; the echoed column is empty, not the token.
	testutils.DoRequestWithHeaders(
		t, base+"/_QUERIES/fulltable/get_auth_header", nil, "GET", http.StatusOK,
		"ExecuteFromScripts", map[string]string{"Authorization": token},
		`[{"?column?": ""}]`,
	)
}

// TestExecuteFromScripts_RejectsUnquotedContextInjection guards the
// unauthenticated read-only SQL injection that survived the CVE-2025-58450 fix:
// sanitizeScriptParam's character allow-list blocks quotes, commas and
// parentheses, but letters, digits and space alone compose
// `0 UNION SELECT <col> FROM <table>` once a template interpolates the value in
// an unquoted context (get_unquoted.read.sql: `WHERE 1 = {{.field1}}`).
// A keyword/comment/cast screen now blanks those payloads, so the query never
// composes and the request fails instead of dumping the catalog.
func TestExecuteFromScripts_RejectsUnquotedContextInjection(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		status      int
	}{
		// Baseline: a plain numeric value still reaches the query and succeeds.
		{
			"GET unquoted-context script with a benign value returns OK",
			"/_QUERIES/fulltable/get_unquoted?field1=1",
			http.StatusOK,
		},
		// UNION arm dumping every column of a table via the whole-row ::text cast.
		{
			"GET unquoted-context script with a UNION payload is neutralized",
			"/_QUERIES/fulltable/get_unquoted?field1=" +
				url.QueryEscape("1 UNION SELECT test7::text FROM test7"),
			http.StatusBadRequest,
		},
		// Role password hashes from the catalog (superuser default in Docker).
		{
			"GET unquoted-context script reading pg_shadow is neutralized",
			"/_QUERIES/fulltable/get_unquoted?field1=" +
				url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
			http.StatusBadRequest,
		},
		// Row-filter bypass needing no UNION at all.
		{
			"GET unquoted-context script with an OR-true row filter bypass is neutralized",
			"/_QUERIES/fulltable/get_unquoted?field1=" + url.QueryEscape("1 OR true"),
			http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, "GET", tc.status, "ExecuteFromScripts")
	}
}

// TestExecuteFromScripts_PreservesSingleTokenValues is the regression test for
// issue #1030. The keyword screen used to blank any value containing a token like
// `do` or `as`, which is a large share of Portuguese slugs — the reporter measured
// 17.5% of a 226k-article catalog. The value came back as '', the query still ran,
// and the endpoint returned HTTP 200 with a row belonging to a different record,
// so callers could not tell that apart from a genuine miss.
//
// A single-token value cannot compose SQL (space is the only separator the
// character allow-list permits), so it now passes through untouched.
func TestExecuteFromScripts_PreservesSingleTokenValues(t *testing.T) {
	base := helpers.ServerURL(t)

	// get_all.read.sql is `SELECT * FROM test7 WHERE name = '{{.field1}}'`. The
	// slug reaches the query verbatim, so it matches nothing rather than matching
	// the empty-name row.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_all?field1=teste-do-abc",
		nil, "GET", http.StatusOK, "ExecuteFromScripts", "[]")

	// Same for the other tokens the reporter hit.
	for _, slug := range []string{"teste-as-abc", "teste-select-abc", "compra-do-mes"} {
		t.Log("slug with an embedded SQL keyword token is preserved: " + slug)
		testutils.DoRequest(t,
			base+"/_QUERIES/fulltable/get_all?field1="+url.QueryEscape(slug),
			nil, "GET", http.StatusOK, "ExecuteFromScripts", "[]")
	}
}

// TestExecuteFromScripts_RejectsInterpolatedRejectedValue covers the other half of
// issue #1030: a value the screen still refuses (multi-word, carrying a keyword)
// must fail the request rather than be blanked into a query that returns the
// wrong rows under HTTP 200.
func TestExecuteFromScripts_RejectsInterpolatedRejectedValue(t *testing.T) {
	base := helpers.ServerURL(t)

	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_all?field1="+url.QueryEscape("compra do mes"),
		nil, "GET", http.StatusBadRequest, "ExecuteFromScripts")
}

// TestExecuteFromScripts_BoundValueSkipsScreen proves the supported escape hatch:
// a template that binds with sqlVal receives the caller's value untouched, because
// a bound value is sent out of band and cannot compose SQL. This is the migration
// path for full-text search and any other free-form input.
func TestExecuteFromScripts_BoundValueSkipsScreen(t *testing.T) {
	base := helpers.ServerURL(t)

	// get_bound.read.sql is `SELECT * FROM test7 WHERE name = {{sqlVal "field1"}}`.
	// The phrase would be blanked if interpolated, yet here it reaches Postgres as
	// a bound parameter and simply matches no row.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1="+url.QueryEscape("compra do mes"),
		nil, "GET", http.StatusOK, "ExecuteFromScripts", "[]")

	// An injection payload is equally inert once bound — it is data, not syntax.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1="+url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
		nil, "GET", http.StatusOK, "ExecuteFromScripts", "[]")

	// And a value that does exist still comes back.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1=gopher",
		nil, "GET", http.StatusOK, "ExecuteFromScripts")
}

// Ordinary browser headers fail the character allow-list (parentheses, semicolons)
// but are only blanked and logged — never a request failure — since every inbound
// header is screened, not just the ones a template reads.
func TestExecuteFromScripts_TolerantOfBrowserHeaders(t *testing.T) {
	base := helpers.ServerURL(t)

	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Referer":    "https://example.com/a/b?c=d&e=f",
	}
	testutils.DoRequestWithHeaders(t,
		base+"/_QUERIES/fulltable/get_all?field1=gopher",
		nil, "GET", http.StatusOK, "ExecuteFromScripts", headers)
}

// TestExecuteFromScripts_RejectsUnregisteredDatabase guards against
// ScriptHandler.Execute reaching the connection layer with an arbitrary,
// attacker-chosen database name via /_QUERIES/{database}/{queriesLocation}/{script}.
// Every other controller validates the database against the registry before
// touching the connection layer; Execute used to skip that check, so
// dbFromCtx would attempt a real outbound Postgres connection for any name in
// the path. It also skipped validatePathSegments entirely, so an unsafe
// queriesLocation/script segment used to reach ResolveScript unchecked.
func TestExecuteFromScripts_RejectsUnregisteredDatabase(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		status      int
	}{
		{
			"GET _QUERIES with an unregistered database name is rejected, not attempted as a connection",
			"/_QUERIES/evil-db-does-not-exist/fulltable/get_all?field1=gopher",
			http.StatusBadRequest,
		},
		{
			"GET _QUERIES with an unsafe script segment is rejected",
			"/_QUERIES/fulltable/evil'name",
			http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, "GET", tc.status, "ExecuteFromScripts")
	}
}

func TestRenderWithXML(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		bodies      []string
	}{
		{
			"Get schemas with COUNT clause with XML Render",
			"/schemas?_count=*&_renderer=xml",
			"GET",
			200,
			[]string{"<objects>", "<count>"},
		},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, "GetSchemas", tc.bodies...)
	}
}

// ---------------------------------------------------------------------------
// Script parameter pipeline (issue #1030 and the review findings that followed)
//
// testutils.DoRequest can only assert that a body CONTAINS a string, which
// cannot express "the payload must not appear". scriptBody returns the raw body
// so containment cases can assert the negative.
// ---------------------------------------------------------------------------

// scriptBody performs the request and returns status and body, asserting nothing.
func scriptBody(t *testing.T, url, method string, headers map[string]string) (int, string) {
	t.Helper()

	req, err := http.NewRequest(method, url, nil)
	require.NoError(t, err)
	req.Header.Set("X-Application", "prest")
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

// injectionPayloads are values that must never reach the SQL text intact. Each is
// checked against the response with NotContains, so they carry a distinctive
// marker (pg_shadow, rolpassword) that could only appear if the payload executed.
var injectionPayloads = []string{
	"1 UNION SELECT passwd FROM pg_shadow",
	"1 UNION SELECT rolpassword::text FROM pg_authid",
	"1 OR true",
}

// TestExecuteFromScripts_RawMapsUnreachableFromTemplate is the E2E half of the
// review finding that `_param` / `_header` sat in the template's dot: a template
// could read them with `{{index ._param "field1"}}` and interpolate the caller's
// UNSCREENED value straight into the SQL, bypassing the screen entirely. A local
// reproduction rendered `WHERE 1 = 0 UNION SELECT passwd FROM pg_shadow` verbatim.
//
// NewFuncRegistry now moves those maps off the template data, so the fixtures
// below cannot reach them. Either outcome is acceptable — a 400 because the
// template failed to render, or a 200 whose body simply lacks the payload — but
// the value must never appear in the response.
func TestExecuteFromScripts_RawMapsUnreachableFromTemplate(t *testing.T) {
	base := helpers.ServerURL(t)

	var fixtures = []struct {
		description string
		script      string
	}{
		{"index into the raw param map", "get_raw_param_index"},
		{"render the whole raw param map", "get_raw_param_dot"},
		{"index into the raw header map", "get_raw_header_index"},
	}

	for _, f := range fixtures {
		for _, payload := range injectionPayloads {
			t.Log(f.description + " with payload: " + payload)
			status, body := scriptBody(t,
				base+"/_QUERIES/fulltable/"+f.script+"?field1="+url.QueryEscape(payload),
				"GET", map[string]string{"X-Application": payload})

			require.NotContains(t, body, "pg_shadow", f.description)
			require.NotContains(t, body, "pg_authid", f.description)
			require.NotContains(t, body, "rolpassword", f.description)
			require.NotContains(t, body, "UNION", f.description)
			require.Contains(t, []int{http.StatusOK, http.StatusBadRequest}, status,
				"unexpected status for %s", f.description)
		}
	}
}

// TestExecuteFromScripts_BoundPlaceholderOrdering checks that two bound values
// arrive as $1 and $2 in the order the template names them, not the map order of
// the query string. Binding the wrong value to the wrong column would silently
// return another record's rows.
func TestExecuteFromScripts_BoundPlaceholderOrdering(t *testing.T) {
	base := helpers.ServerURL(t)

	// get_bound_two.read.sql: WHERE name = {{sqlVal "field1"}} AND surname = {{sqlVal "field2"}}
	// The seeded row is ('gopher', 'da silva'); matching it proves the pairing.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound_two?field1=gopher&field2="+url.QueryEscape("da silva"),
		nil, "GET", http.StatusOK, "BoundOrdering", `"name": "gopher"`)

	// Swapping the values must match nothing — if the binding order were wrong,
	// this would be the query that succeeds.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound_two?field1="+url.QueryEscape("da silva")+"&field2=gopher",
		nil, "GET", http.StatusOK, "BoundOrderingSwapped", "[]")
}

// TestExecuteFromScripts_BoundListValues covers sqlList, which had no coverage at
// all. It is the supported replacement for inFormat: values are bound as
// ($1,$2,...) rather than joined into quoted SQL text.
func TestExecuteFromScripts_BoundListValues(t *testing.T) {
	base := helpers.ServerURL(t)

	// Repeated parameter: both values are bound, so the seeded row matches through
	// the second element.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound_list?field1=nobody&field1=gopher",
		nil, "GET", http.StatusOK, "BoundList", `"name": "gopher"`)

	// Single value: sqlList takes the non-slice branch and still renders ($1).
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound_list?field1=gopher",
		nil, "GET", http.StatusOK, "BoundListSingle", `"name": "gopher"`)

	// A value the inline screen would reject is inert once bound: it matches no
	// row rather than composing SQL or failing the request.
	status, body := scriptBody(t,
		base+"/_QUERIES/fulltable/get_bound_list?field1="+url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
		"GET", nil)
	require.Equal(t, http.StatusOK, status)
	require.NotContains(t, body, "pg_shadow")
}

// TestExecuteFromScripts_BoundHeaderValues covers `{{sqlVal "header.X"}}`, which
// binds the header's UNSCREENED value. The credential case is the one that
// matters: blanking Authorization is about secrecy rather than SQL composition,
// so binding must not become a way to read the caller's token back out.
func TestExecuteFromScripts_BoundHeaderValues(t *testing.T) {
	base := helpers.ServerURL(t)
	token := "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJnb3BoZXIifQ.c2ln"

	// A non-credential header binds its real value, spaces and all — the value
	// would be blanked if it were interpolated instead.
	testutils.DoRequestWithHeaders(t,
		base+"/_QUERIES/fulltable/get_bound_header", nil, "GET", http.StatusOK,
		"BoundHeader", map[string]string{"X-Application": "compra do mes"},
		"compra do mes")

	// The credential header binds empty even though a token was sent.
	_, body := scriptBody(t, base+"/_QUERIES/fulltable/get_bound_auth_header", "GET",
		map[string]string{"Authorization": token})
	require.NotContains(t, body, "eyJhbGciOiJIUzI1NiJ9",
		"a bearer token must never be bindable into a query")
}

// TestExecuteFromScripts_IdentQuotesIdentifiers covers the ident helper, which had
// no coverage. An identifier cannot be bound as a parameter, so ident validates it
// against a strict charset and double-quotes it instead.
func TestExecuteFromScripts_IdentQuotesIdentifiers(t *testing.T) {
	base := helpers.ServerURL(t)

	// A valid table name resolves and the query runs.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_ident?field1=test7",
		nil, "GET", http.StatusOK, "Ident", `"name": "gopher"`)

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
		status, body := scriptBody(t,
			base+"/_QUERIES/fulltable/get_ident?field1="+url.QueryEscape(tc.value), "GET", nil)
		require.Equal(t, http.StatusBadRequest, status, tc.description)

		// The template error names the template internals and the offending value;
		// it must not be reflected back to the caller, matching the policy applied
		// to rejected parameters and to SQL failures.
		require.NotContains(t, body, "DROP TABLE", tc.description)
		require.NotContains(t, body, "invalid identifier", tc.description)
		require.Contains(t, body, "check your prest logs", tc.description)
	}

	// The DDL in the payload above must never have executed: test7 still answers.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_ident?field1=test7",
		nil, "GET", http.StatusOK, "IdentTableIntact", `"name": "gopher"`)
}

// TestExecuteFromScripts_ScreenedCharsetSurvives pins the characters the allow-list
// permits. These are ordinary data — emails, dates, paths — and a regression that
// blanked them would resurface issue #1030 in a different shape.
func TestExecuteFromScripts_ScreenedCharsetSurvives(t *testing.T) {
	base := helpers.ServerURL(t)

	var values = []struct {
		description string
		value       string
	}{
		{"email address", "user@example.com"},
		{"ISO date", "2024-01-01"},
		{"path-like value", "a/b.c"},
		{"identifier prefixed by a keyword", "order66"},
		{"keyword joined by an underscore", "union_member"},
		{"slug carrying a keyword token", "estatua-do-diabo"},
	}
	for _, tc := range values {
		t.Log("value survives the screen: " + tc.description)
		// get_all.read.sql interpolates into a quoted literal, so a surviving value
		// simply matches no row; a blanked one would match the empty-name record.
		testutils.DoRequest(t,
			base+"/_QUERIES/fulltable/get_all?field1="+url.QueryEscape(tc.value),
			nil, "GET", http.StatusOK, "CharsetSurvives", "[]")
	}
}

// TestExecuteFromScripts_RejectionNamesParameterOnly checks the shape of the 400:
// it must identify which parameter was refused so a caller can fix the request,
// and must never echo the value back, which is attacker-controlled and may carry a
// credential (CodeQL go/clear-text-logging).
func TestExecuteFromScripts_RejectionNamesParameterOnly(t *testing.T) {
	base := helpers.ServerURL(t)

	status, body := scriptBody(t,
		base+"/_QUERIES/fulltable/get_all?field1="+url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
		"GET", nil)

	require.Equal(t, http.StatusBadRequest, status)
	require.Contains(t, body, "field1", "the error must name the offending parameter")
	require.NotContains(t, body, "pg_shadow", "the error must not echo the value")
	require.Contains(t, body, "sqlVal", "the error should point at the supported alternative")

	// The body must still be parseable JSON — a message containing quotes used to
	// be interpolated raw and produced a body no client could decode.
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &decoded))
	require.NotEmpty(t, decoded["error"])
}

// TestExecuteFromScripts_ReservedKeysNotCallerControlled guards the collision the
// review surfaced. `header`, `_header` and `_param` are template-data slots pREST
// fills itself; a query parameter used to be able to overwrite them:
//   - ?header=x replaced the screened header map with a string, so every template
//     doing {{index .header "..."}} failed to render — a 400 on demand.
//   - ?_header=x replaced the raw header map, so {{sqlVal "header.X"}} fell back
//     to the screened value and bound "" for any header the charset screen
//     rejects: wrong rows under HTTP 200, the failure class of issue #1030.
func TestExecuteFromScripts_ReservedKeysNotCallerControlled(t *testing.T) {
	base := helpers.ServerURL(t)

	// Inline header interpolation keeps working while the caller tries to clobber
	// the map that backs it.
	testutils.DoRequestWithHeaders(t,
		base+"/_QUERIES/fulltable/get_header?header=x&_header=x&_param=x",
		nil, "GET", http.StatusOK, "ReservedInline",
		map[string]string{"X-Application": "prest"}, `"prest"`)

	// Header binding still reaches the real, unscreened value.
	testutils.DoRequestWithHeaders(t,
		base+"/_QUERIES/fulltable/get_bound_header?_header=x",
		nil, "GET", http.StatusOK, "ReservedBinding",
		map[string]string{"X-Application": "compra do mes"}, "compra do mes")

	// And parameter binding is likewise undisturbed.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound?_param=x&field1=gopher",
		nil, "GET", http.StatusOK, "ReservedParamBinding", `"name": "gopher"`)
}

// TestExecuteFromScripts_BoundWriteVerbs covers the bound path for the verbs that
// mutate: POST, PUT/PATCH and DELETE all share the same screen and binding code
// as GET, but none of them had coverage through sqlVal. The values used here would
// be refused if interpolated (they carry a keyword and a space), which is exactly
// what makes them worth writing with.
//
// test7 is shared mutable state across this suite, so these rows use a marker no
// other test touches and are deleted at the end.
func TestExecuteFromScripts_BoundWriteVerbs(t *testing.T) {
	base := helpers.ServerURL(t)

	const (
		name    = "bound do write"    // rejected inline: keyword "do" plus spaces
		surname = "sobrenome do teste"
		updated = "atualizado do teste"
	)

	// INSERT through bound parameters. rows_affected proves the statement ran with
	// the real values rather than blanks.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/write_bound?field1="+url.QueryEscape(name)+"&field2="+url.QueryEscape(surname),
		nil, "POST", http.StatusOK, "BoundInsert", `"rows_affected":1`)

	// Read it back through a bound SELECT: the row must carry the exact values,
	// which also proves nothing was blanked on the way in.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1="+url.QueryEscape(name),
		nil, "GET", http.StatusOK, "BoundInsertReadback", surname)

	// UPDATE through bound parameters (PUT and PATCH share update_bound.update.sql).
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/update_bound?field1="+url.QueryEscape(name)+"&field2="+url.QueryEscape(updated),
		nil, "PUT", http.StatusOK, "BoundUpdate", `"rows_affected":1`)
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1="+url.QueryEscape(name),
		nil, "GET", http.StatusOK, "BoundUpdateReadback", updated)

	// PATCH resolves to the same template; it must behave identically.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/update_bound?field1="+url.QueryEscape(name)+"&field2="+url.QueryEscape(surname),
		nil, "PATCH", http.StatusOK, "BoundPatch", `"rows_affected":1`)

	// DELETE through a bound parameter, then confirm the row is gone.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/delete_bound?field1="+url.QueryEscape(name),
		nil, "DELETE", http.StatusOK, "BoundDelete", `"rows_affected":1`)
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_bound?field1="+url.QueryEscape(name),
		nil, "GET", http.StatusOK, "BoundDeleteReadback", "[]")
}

// TestExecuteFromScripts_MultiValueParameterMatrix pins how a repeated query
// parameter behaves in each helper. The type handed to the template changes when
// one element is rejected ([]string becomes []interface{} so the marker can report
// itself), and the two list helpers diverge deliberately: inFormat interpolates
// and therefore fails, while sqlList binds and therefore succeeds.
func TestExecuteFromScripts_MultiValueParameterMatrix(t *testing.T) {
	base := helpers.ServerURL(t)

	// All elements survive the screen: inFormat joins them into a quoted IN list.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_all_slice?field1=nobody&field1=gopher",
		nil, "GET", http.StatusOK, "SliceAllSafe", `"name": "gopher"`)

	// One element is rejected. inFormat interpolates, so the marker renders and the
	// request fails rather than silently querying a truncated list.
	status, body := scriptBody(t,
		base+"/_QUERIES/fulltable/get_all_slice?field1=gopher&field1="+url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
		"GET", nil)
	require.Equal(t, http.StatusBadRequest, status,
		"a rejected element must fail the request, not shrink the IN list")
	require.NotContains(t, body, "pg_shadow")

	// The same request against the bound helper succeeds: every element is bound,
	// so the payload is data and matches nothing.
	status, body = scriptBody(t,
		base+"/_QUERIES/fulltable/get_bound_list?field1=gopher&field1="+url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
		"GET", nil)
	require.Equal(t, http.StatusOK, status,
		"bound list values are never screened, so the request must succeed")
	require.Contains(t, body, `"name": "gopher"`)
	require.NotContains(t, body, "passwd")
}

// TestExecuteFromScripts_UnEscapeCannotReintroduceQuotes is adversarial: unEscape
// URL-decodes its argument, so if a percent-escape could survive the screen a
// template using it would decode `%27` back into the quote the screen exists to
// remove. It cannot, because `%` is itself outside the character allow-list — a
// double-encoded payload is refused one level earlier. That is load-bearing and
// undocumented, so it is pinned here.
func TestExecuteFromScripts_UnEscapeCannotReintroduceQuotes(t *testing.T) {
	base := helpers.ServerURL(t)

	// Plain value: unEscape is a no-op on a screened value (no percent-escape can
	// survive the charset screen), so the query behaves like any quoted template.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_unescape?field1=gopher",
		nil, "GET", http.StatusOK, "UnEscapePlain", `"name": "gopher"`)

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
		status, body := scriptBody(t,
			base+"/_QUERIES/fulltable/get_unescape?field1="+url.QueryEscape(tc.value), "GET", nil)
		require.Equal(t, http.StatusBadRequest, status, tc.description)
		require.NotContains(t, body, "pg_shadow", tc.description)
	}
}

// TestExecuteFromScripts_LimitOffsetSwallowsBadPagination documents a sharp edge
// rather than a fix: limitOffset returns an empty string when its arguments do not
// parse, so the whole LIMIT clause disappears and the script returns every row.
// A caller sending ?page=abc turns a paginated endpoint into a full-table read.
// Pinned so the behavior cannot change silently; worth revisiting separately.
func TestExecuteFromScripts_LimitOffsetSwallowsBadPagination(t *testing.T) {
	base := helpers.ServerURL(t)

	// Valid pagination: the clause renders and the query succeeds.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_limitoffset_params?page=1&size=1",
		nil, "GET", http.StatusOK, "LimitOffsetValid", `"name"`)

	// Non-numeric page: the clause vanishes instead of erroring, and every row is
	// returned. This is the documented-but-undesirable behavior.
	status, body := scriptBody(t,
		base+"/_QUERIES/fulltable/get_limitoffset_params?page=abc&size=1", "GET", nil)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `"name"`,
		"limitOffset drops the clause on a parse error, returning unpaginated rows")
}

// TestExecuteFromScripts_DefaultOrValueWithRejectedParam covers an interaction that
// is easy to get wrong: defaultOrValue applies its default only when the key is
// absent, and a rejected value still occupies the key. So a refused parameter does
// not quietly fall back to the default — the request fails, which is the right
// outcome (falling back would answer with rows the caller did not ask for).
func TestExecuteFromScripts_DefaultOrValueWithRejectedParam(t *testing.T) {
	base := helpers.ServerURL(t)

	// No parameter at all: the default applies and the seeded row comes back.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/funcs",
		nil, "GET", http.StatusOK, "DefaultApplied", `"name": "gopher"`)

	// A value that survives the screen overrides the default.
	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/funcs?field1=teste-do-abc",
		nil, "GET", http.StatusOK, "DefaultOverridden", "[]")

	// A rejected value fails rather than silently reverting to "gopher".
	status, body := scriptBody(t,
		base+"/_QUERIES/fulltable/funcs?field1="+url.QueryEscape("1 UNION SELECT passwd FROM pg_shadow"),
		"GET", nil)
	require.Equal(t, http.StatusBadRequest, status)
	require.NotContains(t, body, "gopher",
		"a refused parameter must not fall through to the template default")
}

// TestExecuteFromScripts_EmptyParameterIsNotRejected documents that an explicitly
// empty value (?field1=) is legitimate input, not a rejection: it interpolates as
// an empty literal and the request succeeds. Worth pinning because it is the one
// case that still produces "no rows" without an error, and it must not be confused
// with the blanking bug issue #1030 was about.
func TestExecuteFromScripts_EmptyParameterIsNotRejected(t *testing.T) {
	base := helpers.ServerURL(t)

	testutils.DoRequest(t,
		base+"/_QUERIES/fulltable/get_all?field1=",
		nil, "GET", http.StatusOK, "EmptyParam", "[]")
}
