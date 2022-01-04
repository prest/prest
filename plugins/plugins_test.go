package plugins

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/testutils"
)

func initAuthRoutes() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/_PLUGIN/{file}/{func}", HandlerPlugin)
	return r
}

func TestPlugins(t *testing.T) {
	config.Load()
	postgres.Load()
	// running the tests at this point the working folder will be the plugins
	// package folder, so to return a directory `../`
	config.PrestConf.PluginPath = "../lib"
	server := httptest.NewServer(initAuthRoutes())
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		// should be status 200 `http.StatusOK`, but ... read TODO
		{"/_PLUGIN/hello/Hello request GET method", "/_PLUGIN/hello/Hello", "GET", http.StatusNotFound},
		{"/_PLUGIN/hello/Hello request POST method", "/_PLUGIN/hello/Hello", "POST", http.StatusNotFound},
	}

	// TODO: tests will not work because the plugin system has an error loading at runtime
	// plugin.Open("../lib/hello"): plugin was built with a different version of package runtime
	// ref: https://github.com/golang/go/issues/27751
	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "Plugins")
	}
}
