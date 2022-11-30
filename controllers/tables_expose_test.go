package controllers

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/middlewares"
	"github.com/prest/prest/testutils"
)

func init() {
	fmt.Println("AQUI COMECA")
	os.Setenv("PREST_DEBUG", "true")
	os.Setenv("PREST_CONF", "../testdata/prest_expose.toml")
	os.Setenv("PREST_JWT_DEFAULT", "false")
	config.Load()
	postgres.Load()
	config.PrestConf.Adapter = &postgres.Postgres{}
}

func TestGetTablesWithEnabledExposeMiddleware(t *testing.T) {
	config.Load()
	postgres.Load()
	config.PrestConf.Adapter = &postgres.Postgres{}
	config.PrestConf.ExposeConf.Enabled = true
	config.PrestConf.ExposeConf.TableListing = true
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/tables", GetTables).Methods("GET")
	n := middlewares.GetApp()
	n.UseHandler(router)
	server := httptest.NewServer(n)
	defer server.Close()
	// n := middlewares.GetApp()
	// router := mux.NewRouter()
	// router.HandleFunc("/{database}/{schema}/tables", GetTables).Methods("GET").Name("tables")
	// n.UseHandler(router)
	// server := httptest.NewServer(n)
	// defer server.Close()

	// url, _ := router.Get("tables").URL("database", "prest-test", "schema", "public")

	// if err != nil {
	// 	fmt.Println(err)
	// }

	// fmt.Printf("url: %v", url)

	testutils.DoRequest(t, server.URL+"/prest-test/public/tables", nil, "GET", 401, "GetTables")
	// fmt.Println("AQUI TERMINA")
}
