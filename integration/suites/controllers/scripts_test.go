// nolint
package controllers_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
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
