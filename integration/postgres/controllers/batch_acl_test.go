package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
)

// TestBatchInsert_EnforcesTablePermissions guards the middlewares.getVars gap
// where /batch/{database}/{schema}/{table} (a 5-element path split once the
// leading "/" is accounted for) fell outside the shapes getVars handled and
// returned nil, which AccessControl treats as "not a table path" and lets
// through unconditionally — silently skipping TablePermissions for batch
// inserts regardless of ACL config. BatchInsert itself never checks
// permissions on its own; it relies entirely on the AccessControl middleware.
func TestBatchInsert_EnforcesTablePermissions(t *testing.T) {
	base := helpers.AuthServerURL(t)
	token := helpers.LoginToken(t, base, "test@postgres.rest", "123456")

	var testCases = []struct {
		description string
		url         string
		body        interface{}
		status      int
	}{
		{
			"POST /batch on a read-only table (test_readonly_access has no write permission) is rejected",
			"/batch/prest-test/public/test_readonly_access",
			[]map[string]string{{"name": "batch-acl-poc"}},
			http.StatusUnauthorized,
		},
		{
			"POST /batch on a fully-permissioned table (test5) still succeeds",
			"/batch/prest-test/public/test5",
			[]map[string]string{{"celphone": "batch-acl-ok"}},
			http.StatusCreated,
		},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		helpers.DoAuthRequest(t, base+tc.url, tc.body, http.MethodPost, token, tc.status, "BatchInsertACL")
	}
}

// TestBatchInsert_RequiresAuth is the baseline for the case above: with no
// token at all, AuthMiddleware (which runs before AccessControl in the
// stack) must reject the request before TablePermissions is ever consulted.
func TestBatchInsert_RequiresAuth(t *testing.T) {
	base := helpers.AuthServerURL(t)

	helpers.DoAuthRequest(
		t, base+"/batch/prest-test/public/test5",
		[]map[string]string{{"celphone": "no-token"}},
		http.MethodPost, "", http.StatusUnauthorized, "BatchInsertACL",
	)
}
