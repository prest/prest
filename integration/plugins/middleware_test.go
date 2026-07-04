package plugins_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/plugins"
	"github.com/prest/prest/v2/testutils"
	"github.com/urfave/negroni/v3"
)

func initMiddlewarePluginTestRouter(cfg *config.Prest) *negroni.Negroni {
	cfg.PluginPath = helpers.PluginLibDir()
	cfg.PluginMiddlewareList = []config.PluginMiddleware{
		{File: "hello", Func: "Hello"},
	}
	plg := plugins.New(cfg)
	r := negroni.New()
	r.Use(plg.Middleware())
	return r
}

func TestPluginsMiddleware(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	server := httptest.NewServer(initMiddlewarePluginTestRouter(cfg))
	defer server.Close()

	testutils.DoRequest(t, server.URL+"/", nil, "GET", http.StatusOK, "Plugins")
}
