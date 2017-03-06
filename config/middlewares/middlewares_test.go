package middlewares

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/config/router"
	"github.com/nuveo/prest/controllers"
	"github.com/nuveo/prest/middlewares"
	"github.com/urfave/negroni"
)

func TestInitApp(t *testing.T) {
	app = nil
	initApp()
	if app == nil {
		t.Errorf("App should not be nil.")
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestGetApp(t *testing.T) {
	app = nil
	n := GetApp()
	if n == nil {
		t.Errorf("Should return an app.")
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestGetAppWithReorderedMiddleware(t *testing.T) {
	app = nil
	MiddlewareStack = []negroni.Handler{
		negroni.Handler(negroni.HandlerFunc(customMiddleware)),
	}
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal("Expected run without errors but was", err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Expected run without errors but was", err.Error())
	}
	defer resp.Body.Close()
	if !strings.Contains(string(body), "Calling custom middleware") {
		t.Error("do not contains 'Calling custom middleware'")
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type shouldn't be application/json but was")
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestGetAppWithoutReorderedMiddleware(t *testing.T) {
	app = nil
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal("Expected run without errors but was", err.Error())
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should be application/json but not was", resp.Header.Get("Content-Type"))
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestMiddlewareAccessNoblockingCustomRoutes(t *testing.T) {
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("oi")) })
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)

	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")

	r.PathPrefix("/").Handler(negroni.New(
		middlewares.AccessControl(),
		negroni.Wrap(crudRoutes),
	))
	os.Setenv("PREST_CONF", "../../testdata/prest.toml")
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal("Expected run without errors but was", err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Expected run without errors but was", err.Error())
	}
	defer resp.Body.Close()
	if !strings.Contains(string(body), "oi") {
		t.Error("do not contains 'oi'")
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should be application/json but was")
	}
	resp, err = http.Get(server.URL + "prest/public/test_write_and_delete_access")
	if err != nil {
		t.Fatal("Expected run without errors but was", err.Error())
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Expected run without errors but was", err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("content type should be http.StatusUnauthorized but was %s", resp.Status)
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type shouldn't be application/json but was")
	}
	if !strings.Contains(string(body), "required authorization to table") {
		t.Error("do not contains 'required authorization to table'")
	}
	MiddlewareStack = []negroni.Handler{}
	os.Setenv("PREST_CONF", "")
}

func customMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Write([]byte("Calling custom middleware"))
	next(w, r)
}
