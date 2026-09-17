package plugins_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/plugins"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
)

func initMiddlewarePluginTestRouter(cfg *config.Prest) *negroni.Negroni {
	plg := plugins.New(cfg)
	r := negroni.New()
	r.Use(plg.Middleware())
	r.UseHandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return r
}

func TestPluginsMiddleware(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Go plugins are only supported on Linux")
	}

	pluginRoot := t.TempDir()
	mwDir := filepath.Join(pluginRoot, "middlewares")
	require.NoError(t, os.MkdirAll(mwDir, 0o755))

	src := filepath.Join(helpers.PluginLibDir(), "src", "middlewares", "hello.go")
	out := filepath.Join(mwDir, "hello.so")
	// Match go test -race from testdata/runtest.sh: a plugin built without
	// -race cannot be opened by a race-enabled test binary.
	cmd := exec.Command("go", "build", "-race", "-buildmode=plugin", "-o", out, src)
	cmd.Dir = filepath.Dir(helpers.PluginLibDir())
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	outBytes, err := cmd.CombinedOutput()
	require.NoError(t, err, "build hello middleware plugin: %s", string(outBytes))

	cfg := helpers.LoadTestConfig(t)
	cfg.PluginPath = pluginRoot
	cfg.PluginMiddlewareList = []config.PluginMiddleware{
		{File: "hello", Func: "Hello"},
	}

	server := httptest.NewServer(initMiddlewarePluginTestRouter(cfg))
	t.Cleanup(server.Close)

	// GET / through the Hello plugin middleware.
	// Expected: 200 and X-Hello-Middleware set by lib/src/middlewares/hello.go.
	resp, err := http.Get(server.URL + "/")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "Hello Middleware", resp.Header.Get("X-Hello-Middleware"),
		"middleware plugin must run; a silent no-op would leave this header empty (#948)")
}
