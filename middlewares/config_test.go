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
	"github.com/palevi67/prest/adapters/postgres"
	"github.com/palevi67/prest/config"
	"github.com/palevi67/prest/config/router"
	"github.com/palevi67/prest/controllers"
	"github.com/urfave/negroni"
)

func init() {
	config.Load()
	postgres.Load()
}

func TestInitApp(t *testing.T) {
	app = nil
	initApp()
	if app == nil {
		t.Errorf("app should not be nil")
	}
	MiddlewareStack = []negroni.Handler{}
}

func TestGetApp(t *testing.T) {
	app = nil
	n := GetApp()
	if n == nil {
		t.Errorf("should return an app")
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
		t.Fatal("expected run without errors but was", err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
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
	postgres.Load()
	app = nil
	r := router.Get()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)

	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")

	r.PathPrefix("/").Handler(negroni.New(
		AccessControl(),
		negroni.Wrap(crudRoutes),
	))
	os.Setenv("PREST_CONF", "../testdata/prest.toml")
	n := GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
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
		t.Fatal("expected run without errors but was", err.Error())
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("expected run without errors but was", err.Error())
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

func TestEnableDefaultJWT(t *testing.T) {
	app = nil
	os.Setenv("PREST_JWT_DEFAULT", "false")
	os.Setenv("PREST_DEBUG", "false")
	config.Load()
	nd := appTest()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()
	respd, err := http.Get(serverd.URL)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusNotImplemented {
		t.Errorf("expected status code 501, but got %d", respd.StatusCode)
	}
}

func TestJWTIsRequired(t *testing.T) {
	MiddlewareStack = []negroni.Handler{}
	app = nil
	os.Setenv("PREST_JWT_DEFAULT", "true")
	os.Setenv("PREST_DEBUG", "false")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	respd, err := http.Get(serverd.URL)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status code 401, but got %d", respd.StatusCode)
	}
}

func TestJWTSignatureOk(t *testing.T) {
	app = nil
	MiddlewareStack = nil
	bearer := "Bearer eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJpYXQiOjE1MTc1NjM2MTYsImlzcyI6InByaXZhdGUiLCJqdGkiOiJjZWZhNzRmZS04OTRjLWZmNjMtZDgxNi00NjIwYjhjZDkyZWUiLCJvcmciOiJwcml2YXRlIiwic3ViIjoiam9obi5kb2UifQ.zLWkEd4hP4XdCD_DlRy6mgPeKwEl1dcdtx5A_jHSfmc87EsrGgNSdi8eBTzCgSU0jgV6ssTgQwzY6x4egze2xA"
	os.Setenv("PREST_JWT_DEFAULT", "true")
	os.Setenv("PREST_DEBUG", "false")
	os.Setenv("PREST_JWT_KEY", "s3cr3t")
	os.Setenv("PREST_JWT_ALGO", "HS512")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, but got %d", respd.StatusCode)
	}
}

func TestJWTSignatureKo(t *testing.T) {
	app = nil
	bearer := "Bearer: eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJleHAiOjE1MjUzMzk2MTYsImlhdCI6MTUxNzU2MzYxNiwiaXNzIjoicHJpdmF0ZSIsImp0aSI6ImNlZmE3NGZlLTg5NGMtZmY2My1kODE2LTQ2MjBiOGNkOTJlZSIsIm9yZyI6InByaXZhdGUiLCJzdWIiOiJqb2huLmRvZSJ9.zGP1Xths2bK2r9FN0Gv1SzyoisO0dhRwvqrPvunGxUyU5TbkfdnTcQRJNYZzJfGILeQ9r3tbuakWm-NIoDlbbA"
	os.Setenv("PREST_JWT_DEFAULT", "true")
	os.Setenv("PREST_DEBUG", "false")
	os.Setenv("PREST_JWT_KEY", "s3cr3t")
	os.Setenv("PREST_JWT_ALGO", "HS256")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if respd.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status code 401, but got %d", respd.StatusCode)
	}
}

func appTest() *negroni.Negroni {
	n := GetApp()
	r := router.Get()
	if !config.PrestConf.Debug && !config.PrestConf.EnableDefaultJWT {
		n.UseHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
	}
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}

func appTestWithJwt() *negroni.Negroni {
	n := GetApp()
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}

func TestCors(t *testing.T) {
	MiddlewareStack = []negroni.Handler{}
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
	req, err := http.NewRequest("OPTIONS", server.URL, nil)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	req.Header.Set("Access-Control-Request-Method", "GET")
	client := http.Client{}
	resp, err := client.Do(req)
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
	if resp.Request.Method != "OPTIONS" {
		t.Errorf("expected method OPTIONS, but got %v", resp.Request.Method)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected HTTP status code 200, but got %v", resp.StatusCode)
	}
	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal("expected run without errors but was", err)
	}
	if len(body) != 0 {
		t.Error("body is not empty")
	}
}
