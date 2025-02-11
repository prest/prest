package plugins

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/testutils"

	"github.com/urfave/negroni/v3"
)

func initMiddlewarePluginTestRouter() *negroni.Negroni {
	r := negroni.New()
	r.Use(MiddlewarePlugin())
	return r
}

func TestPluginsMiddleware(t *testing.T) {
	config.Load()
	postgres.Load()
	config.PrestConf.PluginPath = "../lib"
	config.PrestConf.PluginMiddlewareList = []config.PluginMiddleware{
		{
			File: "hello",
			Func: "Hello",
		},
	}
	server := httptest.NewServer(initMiddlewarePluginTestRouter())
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		// TODO: should be status 200 `http.StatusOK`, but read
		{"/", "/", "GET", http.StatusOK},
	}

	// TODO: tests will not work because the plugin system has an error loading at runtime
	// plugin.Open("../lib/hello"): plugin was built with a different version of package runtime
	// ref: https://github.com/golang/go/issues/27751
	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "Plugins")
	}
}
