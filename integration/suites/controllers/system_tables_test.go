package controllers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

// TestSystemTables_DeniedThroughGenericRoutes guards GHSA-2x9q-r7j9-g85f.
// pREST's own control-plane tables (prest_queries holds stored SQL that
// /_QUERIES executes, prest_users holds credentials) used to be reachable
// through the generic table routes: with the advisory default
// access.restrict=false, TableAllowed approved every table, so an
// unauthenticated caller could insert a stored query and then run it through
// /_QUERIES (RCE) or read credentials back. TableAllowed now denies these
// names (case-insensitive, any schema) in every configuration, so
// AccessControl answers 401 before any SQL runs.
func TestSystemTables_DeniedThroughGenericRoutes(t *testing.T) {
	base := helpers.ServerURL(t)
	body := map[string]string{"name": "ghsa-2x9q"}
	copyHeader := map[string]string{"Prest-Batch-Method": "copy"}

	var testCases = []struct {
		description string
		url         string
		method      string
		body        interface{}
		headers     map[string]string
	}{
		{"GET on prest_queries is denied", "/%s/public/prest_queries", http.MethodGet, nil, nil},
		{"POST on prest_queries is denied", "/%s/public/prest_queries", http.MethodPost, body, nil},
		{"PUT on prest_queries is denied", "/%s/public/prest_queries?name=$eq.x", http.MethodPut, body, nil},
		{"PATCH on prest_queries is denied", "/%s/public/prest_queries?name=$eq.x", http.MethodPatch, body, nil},
		{"DELETE on prest_queries is denied", "/%s/public/prest_queries?name=$eq.x", http.MethodDelete, nil, nil},
		{"GET on prest_users is denied", "/%s/public/prest_users", http.MethodGet, nil, nil},
		{"POST on prest_users is denied", "/%s/public/prest_users", http.MethodPost, body, nil},
		{"PUT on prest_users is denied", "/%s/public/prest_users?username=$eq.x", http.MethodPut, body, nil},
		{"PATCH on prest_users is denied", "/%s/public/prest_users?username=$eq.x", http.MethodPatch, body, nil},
		{"DELETE on prest_users is denied", "/%s/public/prest_users?username=$eq.x", http.MethodDelete, nil, nil},
		{"upper-case PREST_QUERIES path is denied (case-insensitive match)", "/%s/public/PREST_QUERIES", http.MethodGet, nil, nil},
		{"mixed-case Prest_Users path is denied (case-insensitive match)", "/%s/public/Prest_Users", http.MethodPost, body, nil},
		{"batch insert into prest_queries is denied", "/batch/%s/public/prest_queries", http.MethodPost, []map[string]string{body}, nil},
		{"batch insert into prest_users is denied", "/batch/%s/public/prest_users", http.MethodPost, []map[string]string{body}, nil},
		{"batch COPY insert into prest_queries is denied", "/batch/%s/public/prest_queries", http.MethodPost, []map[string]string{body}, copyHeader},
		{"batch COPY insert into prest_users is denied", "/batch/%s/public/prest_users", http.MethodPost, []map[string]string{body}, copyHeader},
	}

	for _, db := range helpers.Databases() {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s/%s", db, tc.description), func(t *testing.T) {
				// Expected 401: AccessControl rejects the reserved table
				// even though access.restrict=false on this server.
				testutils.DoRequestWithHeaders(
					t, fmt.Sprintf(base+tc.url, db), tc.body, tc.method,
					http.StatusUnauthorized, "SystemTablesDenied", tc.headers)
			})
		}
	}
}

// TestSystemTables_DeniedAsJoinTarget guards the read-bypass sibling of
// GHSA-2x9q-r7j9-g85f: AccessControl only checks the table in the URL path, so
// an ordinary, permitted table could still pull rows from a reserved table
// through ?_join=. FieldsPermissions now rejects the join target and the
// Select handler answers 400.
func TestSystemTables_DeniedAsJoinTarget(t *testing.T) {
	base := helpers.ServerURL(t)

	var testCases = []struct {
		description string
		url         string
		status      int
	}{
		{"join to prest_queries is rejected", "/%s/public/test?_join=inner:prest_queries:prest_queries.name:$eq:test.name", http.StatusBadRequest},
		{"join to prest_users is rejected", "/%s/public/test?_join=inner:prest_users:prest_users.username:$eq:test.name", http.StatusBadRequest},
		{"join to schema-qualified public.prest_users is rejected", "/%s/public/test?_join=inner:public.prest_users:public.prest_users.username:$eq:test.name", http.StatusBadRequest},
		{"join to upper-case PREST_QUERIES is rejected", "/%s/public/test?_join=left:PREST_QUERIES:PREST_QUERIES.name:$eq:test.name", http.StatusBadRequest},
		{"baseline: join to an ordinary table still works", "/%s/public/test?_join=inner:test8:test8.nameforjoin:$eq:test.name", http.StatusOK},
		{"baseline: plain select on an ordinary table still works", "/%s/public/test", http.StatusOK},
	}

	for _, db := range helpers.Databases() {
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s/%s", db, tc.description), func(t *testing.T) {
				testutils.DoRequest(
					t, fmt.Sprintf(base+tc.url, db), nil, http.MethodGet,
					tc.status, "SystemTablesJoin")
			})
		}
	}
}
