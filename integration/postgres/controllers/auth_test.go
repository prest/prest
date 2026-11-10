package controllers_test

import (
	"net/http"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

func TestAuthEnable(t *testing.T) {
	base := helpers.AuthServerURL(t)

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"GET /auth returns MethodNotAllowed when auth is enabled", "/auth", "GET", http.StatusMethodNotAllowed},
		{"POST /auth without credentials returns Unauthorized", "/auth", "POST", http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, base+tc.url, nil, tc.method, tc.status, tc.description)
	}
}

// TestAuthTokenTTL: the auth service runs with PREST_AUTH_TOKEN_TTL=45m, so a
// token issued by POST /auth must expire 45 minutes after it becomes valid
// instead of after the 6h default.
func TestAuthTokenTTL(t *testing.T) {
	const jwtKey = "integration-test-secret-key-32b!!"
	token := helpers.LoginToken(t, helpers.AuthServerURL(t), "test@postgres.rest", "123456")

	parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.HS256})
	require.NoError(t, err)
	var claims auth.Claims
	require.NoError(t, parsed.Claims([]byte(jwtKey), &claims))
	require.NotNil(t, claims.NotBefore)
	require.NotNil(t, claims.Expiry)
	require.Equal(t, 45*time.Minute, claims.Expiry.Time().Sub(claims.NotBefore.Time()))
}
