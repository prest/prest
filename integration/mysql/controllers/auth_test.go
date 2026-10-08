package mysql_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestMySQLAuthLogin(t *testing.T) {
	// The seeded bcrypt user logs in; the lookup uses backticks and ? binds.
	base := helpers.AuthServerURL(t)
	testutils.DoRequest(t, base+"/auth", map[string]any{"username": "ada", "password": "s3cret"}, http.MethodPost, http.StatusOK, "login", `"token"`, `"username":"ada"`)
}

func TestMySQLAuthRejectsBadCredentials(t *testing.T) {
	// A wrong password and an unknown user are both 401, not a SQL error.
	base := helpers.AuthServerURL(t)
	testutils.DoRequest(t, base+"/auth", map[string]any{"username": "ada", "password": "nope"}, http.MethodPost, http.StatusUnauthorized, "bad password")
	testutils.DoRequest(t, base+"/auth", map[string]any{"username": "nobody", "password": "s3cret"}, http.MethodPost, http.StatusUnauthorized, "unknown user", "user not found")
}
