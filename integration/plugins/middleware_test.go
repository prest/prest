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

func initMiddlewarePluginTestRouter() *negroni.Negroni {
	r := negroni.New()
	r.Use(plugins.MiddlewarePlugin())
	return r
}

func TestPluginsMiddleware(t *testing.T) {
	helpers.LoadTestConfig(t)
	config.PrestConf.PluginPath = helpers.PluginLibDir()
	config.PrestConf.PluginMiddlewareList = []config.PluginMiddleware{
		{File: "hello", Func: "Hello"},
	}
	server := httptest.NewServer(initMiddlewarePluginTestRouter())
	defer server.Close()

	testutils.DoRequest(t, server.URL+"/", nil, "GET", http.StatusOK, "Plugins")
}
