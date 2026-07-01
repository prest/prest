package controllers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/testutils"
)

func TestMultiClusterSelect(t *testing.T) {
	if helpers.SecondaryClusterHost() == "" {
		t.Skip("secondary postgres cluster not configured")
	}

	helpers.LoadMultiClusterConfig(t)
	defer func() { helpers.LoadTestConfig(t) }()

	h := controllers.NewHandlersFromConfig(config.PrestConf)
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", helpers.WithHTTPTimeout(h.CRUD.Select)).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, db := range helpers.Databases() {
		url := fmt.Sprintf("%s/%s/public/test", server.URL, db)
		testutils.DoRequest(t, url, nil, "GET", http.StatusOK, "MultiClusterSelect")
	}
}
