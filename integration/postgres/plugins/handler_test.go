package plugins_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/prest/prest/v2/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initPluginRoutes(cfg *config.Prest) *mux.Router {
	plg := plugins.New(cfg)
	r := mux.NewRouter()
	r.HandleFunc("/_PLUGIN/{file}/{func}", plg.Handler())
	return r
}

func TestPlugins(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	cfg.PluginPath = helpers.PluginLibDir()
	server := httptest.NewServer(initPluginRoutes(cfg))
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"GET plugin Hello returns NotFound when lib is missing or unloadable", "/_PLUGIN/hello/Hello", "GET", http.StatusNotFound},
		{"GET plugin HelloWithStatus returns NotFound when lib is missing or unloadable", "/_PLUGIN/hello/HelloWithStatus", "GET", http.StatusNotFound},
		{"POST plugin Hello returns NotFound when lib is missing or unloadable", "/_PLUGIN/hello/Hello", "POST", http.StatusNotFound},
		// GHSA-fmr6-9j48-xrqp: names that could escape pluginpath or are not
		// Go identifiers are rejected before any file lookup.
		{"GET plugin file with a dot is rejected as invalid name", "/_PLUGIN/hello.evil/Hello", "GET", http.StatusBadRequest},
		{"GET plugin func starting with a digit is rejected as invalid name", "/_PLUGIN/hello/1bad", "GET", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, tc.description)
	}
}

// TestPluginsDotDotFileNeverServed guards GHSA-fmr6-9j48-xrqp. gorilla/mux
// cleans "/_PLUGIN/%2E%2E/Hello" to "/Hello" and redirects, so the handler may
// never see file=".."; either way the request must not succeed.
func TestPluginsDotDotFileNeverServed(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	cfg.PluginPath = helpers.PluginLibDir()
	server := httptest.NewServer(initPluginRoutes(cfg))
	defer server.Close()

	// Encoded dot-dot as the plugin file name.
	// Expected: any status other than 200 (400 if routed, 404 after mux cleaning).
	resp, err := http.Get(server.URL + "/_PLUGIN/%2E%2E/Hello")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

// TestPluginsSymlinkEscapeRejected guards GHSA-fmr6-9j48-xrqp: a validly named
// hello.so inside pluginpath that is a symlink to a library outside it passes
// the name and lexical checks, so only the resolved-path check can stop it.
func TestPluginsSymlinkEscapeRejected(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "evil.so")
	require.NoError(t, os.WriteFile(outside, nil, 0o600))
	pluginDir := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(pluginDir, "hello.so")))

	cfg := helpers.LoadTestConfig(t)
	cfg.PluginPath = pluginDir
	server := httptest.NewServer(initPluginRoutes(cfg))
	defer server.Close()

	// hello.so resolves outside pluginpath.
	// Expected: 400 invalid plugin name, before the library is ever opened.
	testutils.DoRequest(t, server.URL+"/_PLUGIN/hello/Hello", nil, http.MethodGet,
		http.StatusBadRequest, "symlink escaping pluginpath is rejected")
}
