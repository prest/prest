package helpers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

type loginResponse struct {
	Token string `json:"token"`
}

// LoginToken authenticates against /auth and returns the JWT.
func LoginToken(t *testing.T, baseURL, username, password string) string {
	t.Helper()
	baseURL = strings.TrimRight(baseURL, "/")
	payload, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/auth", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "login failed")

	var out loginResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	require.NotEmpty(t, out.Token)
	return out.Token
}

// DoAuthRequest sends an HTTP request with an optional Bearer token.
// If the expectedStatus is 0, the request is expected to fail.
// If the expectedStatus is not 0, the request is expected to succeed and the response body is expected to be in the expectedBody slice.
// If the expectedBody is provided, the request is expected to return the body in the expectedBody slice.
func DoAuthRequest(
	t *testing.T,
	url string,
	r interface{},
	method, token string,
	expectedStatus int,
	name string,
	expectedBody ...string) {

	t.Helper()
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	testutils.DoRequestWithHeaders(
		t, url, r, method, expectedStatus, name, headers, expectedBody...)
}
