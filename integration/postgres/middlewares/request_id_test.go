package middlewares_test

import (
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/stretchr/testify/require"
)

// getRequestID sends GET base+path with the given request ID (none when
// empty) and Host (the URL's host when empty), and returns the status and
// the X-Request-Id response header.
func getRequestID(t *testing.T, base, path, id, host string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	require.NoError(t, err)
	if id != "" {
		req.Header.Set("X-Request-Id", id)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, resp.Header.Get("X-Request-Id")
}

// Server started with PREST_HTTP_REQUEST_ID_HEADER=X-Request-Id: every
// response carries a request ID, the client's when it sent a usable one and a
// generated one otherwise, error responses included.
func TestRequestID_DeployedServer(t *testing.T) {
	base := helpers.RequestIDServerURL(t)
	generated := regexp.MustCompile(`^[0-9a-f]{32}$`)

	t.Run("client ID is sent back", func(t *testing.T) {
		status, id := getRequestID(t, base, "/databases", "client-req-1", "")
		require.Equal(t, http.StatusOK, status)
		require.Equal(t, "client-req-1", id)
	})

	t.Run("an ID is generated when the client sends none", func(t *testing.T) {
		status, first := getRequestID(t, base, "/databases", "", "")
		require.Equal(t, http.StatusOK, status)
		require.Regexp(t, generated, first)

		_, second := getRequestID(t, base, "/databases", "", "")
		require.Regexp(t, generated, second)
		require.NotEqual(t, first, second)
	})

	t.Run("an unusable client ID is replaced", func(t *testing.T) {
		status, id := getRequestID(t, base, "/databases", "has space", "")
		require.Equal(t, http.StatusOK, status)
		require.Regexp(t, generated, id)
	})

	t.Run("a statement error carries the ID", func(t *testing.T) {
		status, id := getRequestID(t, base, "/prest-test/public/missing_table", "client-req-2", "")
		require.Equal(t, http.StatusNotFound, status)
		require.Equal(t, "client-req-2", id)
	})

	t.Run("a refused Host carries the ID", func(t *testing.T) {
		status, id := getRequestID(t, base, "/databases", "client-req-3", "evil.example.com")
		require.Equal(t, http.StatusBadRequest, status)
		require.Equal(t, "client-req-3", id)
	})
}
