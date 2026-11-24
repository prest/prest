package controllers_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/stretchr/testify/require"
)

func TestStudioIndexServesHTML(t *testing.T) {
	t.Parallel()
	base := helpers.ServerURL(t)

	// Fetch the embedded Studio SPA entrypoint.
	// Expected to succeed with text/html (not application/json) so browsers can render it.
	// HandlerSet must not rewrite Content-Type for /_studio paths.
	resp, err := http.Get(base + "/_studio/")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode, "StudioIndexServesHTML")
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(strings.ToLower(string(body)), "<!doctype html>"),
		"expected HTML doctype in body")
}

func TestStudioMissingAssetIs404(t *testing.T) {
	t.Parallel()
	base := helpers.ServerURL(t)

	// Request a missing asset and a missing image under /_studio.
	// Expected 404 (not the SPA index.html with 200).
	// Browsers and proxies must not cache HTML as a JS/PNG response.
	for _, p := range []string{"/_studio/assets/nope.js", "/_studio/nope.png"} {
		resp, err := http.Get(base + p)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusNotFound, resp.StatusCode, p)
	}
}
