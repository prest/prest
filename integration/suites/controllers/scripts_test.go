// nolint
package controllers_test

import (
	"net/http"
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
