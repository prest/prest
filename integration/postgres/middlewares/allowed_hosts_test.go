package middlewares_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/stretchr/testify/require"
)

// getWithHost sends GET base+path with the Host header set to host (the
// request still goes to base's address) and returns the status and body.
func getWithHost(t *testing.T, base, path, host string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	require.NoError(t, err)
	req.Host = host
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, string(body)
}

// Server started with PREST_HTTP_ALLOWED_HOSTS naming its own host and
// 127.0.0.1: requests naming one of those hosts are served, any other Host is
// refused with a 400 before routing, on data and health routes alike.
func TestAllowedHosts_DeployedServer(t *testing.T) {
	base := helpers.AllowedHostsServerURL(t)
	u, err := url.Parse(base)
	require.NoError(t, err)

	cases := []struct {
		description string
		path        string
		host        string
		wantStatus  int
	}{
		{
			description: "configured host with port is served",
			path:        "/databases",
			host:        u.Host,
			wantStatus:  http.StatusOK,
		},
		{
			description: "configured host is matched case-insensitively",
			path:        "/databases",
			host:        strings.ToUpper(u.Hostname()),
			wantStatus:  http.StatusOK,
		},
		{
			description: "configured IP address is served",
			path:        "/_health",
			host:        "127.0.0.1:" + u.Port(),
			wantStatus:  http.StatusOK,
		},
		{
			description: "unlisted host is refused on a data route",
			path:        "/databases",
			host:        "evil.example.com",
			wantStatus:  http.StatusBadRequest,
		},
		{
			description: "unlisted host is refused on the health route",
			path:        "/_health",
			host:        "evil.example.com:" + u.Port(),
			wantStatus:  http.StatusBadRequest,
		},
		{
			description: "subdomain of a configured exact host is refused",
			path:        "/databases",
			host:        "www." + u.Hostname(),
			wantStatus:  http.StatusBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			status, body := getWithHost(t, base, tc.path, tc.host)
			require.Equal(t, tc.wantStatus, status, body)
			if tc.wantStatus != http.StatusBadRequest {
				return
			}
			var payload map[string]string
			require.NoError(t, json.Unmarshal([]byte(body), &payload), body)
			require.Equal(t, "invalid Host header", payload["error"])
		})
	}
}
