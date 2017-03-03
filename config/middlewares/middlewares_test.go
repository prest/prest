package middlewares

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
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

func customMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Write([]byte("Calling custom middleware"))
	next(w, r)
}
