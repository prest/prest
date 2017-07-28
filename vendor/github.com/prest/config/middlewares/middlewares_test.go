package middlewares

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/config"
	"github.com/prest/config/router"
	"github.com/prest/controllers"
	"github.com/prest/middlewares"
	"github.com/urfave/negroni"
)

func init() {
	config.Load()
}

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
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should application/json but wasn't")
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
	os.Setenv("PREST_DEBUG", "true")
	config.Load()
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)

	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")

	r.PathPrefix("/").Handler(negroni.New(
		middlewares.AccessControl(),
		negroni.Wrap(crudRoutes),
	))
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
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
	if !strings.Contains(string(body), "custom route") {
		t.Error("do not contains 'custom route'")
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should be application/json but was", resp.Header.Get("Content-Type"))
	}
	resp, err = http.Get(server.URL + "/prest/public/test_write_and_delete_access")
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
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		t.Error("content type should be application/json but wasn't")
	}
	if !strings.Contains(string(body), "required authorization to table") {
		t.Error("do not contains 'required authorization to table'")
	}
	MiddlewareStack = []negroni.Handler{}
	os.Setenv("PREST_CONF", "")
}

func customMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	m := make(map[string]string)
	m["msg"] = "Calling custom middleware"
	b, _ := json.Marshal(m)

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)

	next(w, r)
}

func TestDebug(t *testing.T) {
	os.Setenv("PREST_DEBUG", "true")
	config.Load()
	nd := appTest()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()
	respd, err := http.Get(serverd.URL)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, but got %d", respd.StatusCode)
	}
}

func appTest() *negroni.Negroni {
	n := GetApp()
	r := router.Get()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}

func TestCorsGet(t *testing.T) {
	os.Setenv("PREST_DEBUG", "true")
	os.Setenv("PREST_CORS_ALLOWORIGIN", "*")
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	config.Load()
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected allow origin *, but got %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if !strings.Contains(methods, method) {
			t.Errorf("do not contain %s", method)
		}
	}
	if resp.Request.Method != "GET" {
		t.Errorf("expected method GET, but got %v", resp.Request.Method)
	}
	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Expected run without errors but was", err)
	}
	if len(body) == 0 {
		t.Error("body is empty")
	}
}

func TestCorsPost(t *testing.T) {
	os.Setenv("PREST_DEBUG", "true")
	os.Setenv("PREST_CORS_ALLOWORIGIN", "*")
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	config.Load()
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Post(server.URL, "application/json", nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected allow origin *, but got %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if !strings.Contains(methods, method) {
			t.Errorf("do not contain %s", method)
		}
	}
	if resp.Request.Method != "POST" {
		t.Errorf("expected method POST, but got %v", resp.Request.Method)
	}
	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Expected run without errors but was", err)
	}
	if len(body) == 0 {
		t.Error("body is empty")
	}
}

func TestCorsHead(t *testing.T) {
	os.Setenv("PREST_DEBUG", "true")
	os.Setenv("PREST_CORS_ALLOWORIGIN", "*")
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	config.Load()
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	client := http.Client{}
	req, err := http.NewRequest("HEAD", server.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	var resp *http.Response
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected allow origin *, but got %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if !strings.Contains(methods, method) {
			t.Errorf("do not contain %s", method)
		}
	}
	if resp.Request.Method != "HEAD" {
		t.Errorf("expected method HEAD, but got %v", resp.Request.Method)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status code 403 but got %d", resp.StatusCode)
	}
}

func TestCorsAllowOriginNotAll(t *testing.T) {
	os.Setenv("PREST_DEBUG", "true")
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	os.Setenv("PREST_CORS_ALLOWORIGIN", "http://127.0.0.1")
	config.Load()
	config.PrestConf.CORSAllowOrigin = []string{"http://127.0.0.1"}
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	client := http.Client{}
	req, err := http.NewRequest("POST", server.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	req.Header.Set("Origin", "http://127.0.0.1")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "http://127.0.0.1" {
		t.Errorf("expected allow origin http://127.0.0.1, but got %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	methods := resp.Header.Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		if !strings.Contains(methods, method) {
			t.Errorf("do not contain %s", method)
		}
	}
	if resp.Request.Method != "POST" {
		t.Errorf("expected method POST, but got %v", resp.Request.Method)
	}
	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("Expected run without errors but was", err)
	}
	if len(body) == 0 {
		t.Error("body is empty")
	}
}
