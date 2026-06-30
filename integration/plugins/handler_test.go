package plugins_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/plugins"
	"github.com/prest/prest/v2/testutils"
)

func initPluginRoutes() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/_PLUGIN/{file}/{func}", plugins.HandlerPlugin)
	return r
}

func TestPlugins(t *testing.T) {
	helpers.LoadTestConfig(t)
	config.PrestConf.PluginPath = "../lib"
	server := httptest.NewServer(initPluginRoutes())
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"/_PLUGIN/hello/Hello request GET method", "/_PLUGIN/hello/Hello", "GET", http.StatusNotFound},
		{"/_PLUGIN/hello/HelloWithStatus request GET method", "/_PLUGIN/hello/HelloWithStatus", "GET", http.StatusNotFound},
		{"/_PLUGIN/hello/Hello request POST method", "/_PLUGIN/hello/Hello", "POST", http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "Plugins")
	}
}
