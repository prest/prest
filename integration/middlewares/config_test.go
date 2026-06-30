package middlewares_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/middlewares"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
)

func TestMain(m *testing.M) {
	helpers.EnsureTestConfigEnv()
	config.Load()
	postgres.Load()
	os.Exit(m.Run())
}

func TestInitApp(t *testing.T) {
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	require.NotNil(t, middlewares.GetApp())
}

func TestGetApp(t *testing.T) {
	helpers.LoadTestConfig(t)
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	require.NotNil(t, middlewares.GetApp())
}

func TestGetAppWithReorderedMiddleware(t *testing.T) {
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	middlewares.MiddlewareStack = []negroni.Handler{
		negroni.Handler(negroni.HandlerFunc(customMiddleware)),
	}
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n := middlewares.GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()
	require.Contains(t, string(body), "Calling custom middleware")
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetAppWithoutReorderedMiddleware(t *testing.T) {
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n := middlewares.GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

func Test_Middleware_DoesntBlock_CustomRoutes(t *testing.T) {
	t.Setenv("PREST_DEBUG", "true")
	helpers.EnsureTestConfigEnv()
	config.Load()
	postgres.Load()
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	h := controllers.NewHandlersFromConfig(config.PrestConf)
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", h.CRUD.Select).Methods("GET")

	r.PathPrefix("/").Handler(negroni.New(
		middlewares.AccessControl(config.PrestConf.Adapter),
		negroni.Wrap(crudRoutes),
	))
	n := middlewares.GetApp()
	n.UseHandler(r)

	server := httptest.NewServer(n)
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Contains(t, string(body), "custom route")
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	resp, err = http.Get(server.URL + "/prest-test/public/test_write_and_delete_access")
	require.NoError(t, err)

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	require.Contains(t, string(body), "authorization required")
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
	t.Setenv("PREST_DEBUG", "true")
	config.Load()
	nd := appTest()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()
	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestEnableDefaultJWT(t *testing.T) {
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	t.Setenv("PREST_JWT_DEFAULT", "false")
	t.Setenv("PREST_DEBUG", "false")
	config.Load()
	nd := appTest()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()
	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
}

func TestJWTIsRequired(t *testing.T) {
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWTSignatureOk(t *testing.T) {
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	middlewares.MiddlewareStack = nil
	bearer := "Bearer eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJpYXQiOjE1MTc1NjM2MTYsImlzcyI6InByaXZhdGUiLCJqdGkiOiJjZWZhNzRmZS04OTRjLWZmNjMtZDgxNi00NjIwYjhjZDkyZWUiLCJvcmciOiJwcml2YXRlIiwic3ViIjoiam9obi5kb2UifQ.zLWkEd4hP4XdCD_DlRy6mgPeKwEl1dcdtx5A_jHSfmc87EsrGgNSdi8eBTzCgSU0jgV6ssTgQwzY6x4egze2xA"
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS512")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respd.StatusCode)
}

func TestJWTSignatureKo(t *testing.T) {
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	bearer := "Bearer: eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJleHAiOjE1MjUzMzk2MTYsImlhdCI6MTUxNzU2MzYxNiwiaXNzIjoicHJpdmF0ZSIsImp0aSI6ImNlZmE3NGZlLTg5NGMtZmY2My1kODE2LTQ2MjBiOGNkOTJlZSIsIm9yZyI6InByaXZhdGUiLCJzdWIiOiJqb2huLmRvZSJ9.zGP1Xths2bK2r9FN0Gv1SzyoisO0dhRwvqrPvunGxUyU5TbkfdnTcQRJNYZzJfGILeQ9r3tbuakWm-NIoDlbbA"
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS256")
	config.Load()
	nd := appTestWithJwt()
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, respd.StatusCode)
}

func appTest() *negroni.Negroni {
	n := middlewares.GetApp()
	r := mux.NewRouter()
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
	n := middlewares.GetApp()
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}

func Test_CORS_Middleware(t *testing.T) {
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	t.Setenv("PREST_DEBUG", "true")
	t.Setenv("PREST_CORS_ALLOWORIGIN", "*")
	t.Setenv("PREST_CONF", helpers.TestConfigPath())
	config.Load()
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	n := middlewares.GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	req, err := http.NewRequest("OPTIONS", server.URL, nil)
	require.NoError(t, err)

	req.Header.Set("Access-Control-Request-Method", "GET")

	client := http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)

	require.Equal(t, "OPTIONS", resp.Request.Method)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	var body []byte
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Zero(t, len(body))
}

func TestExposeTablesMiddleware(t *testing.T) {
	helpers.LoadTestConfig(t)
	middlewares.ResetForTest()
	t.Cleanup(middlewares.ResetForTest)
	t.Setenv("PREST_DEBUG", "true")
	t.Setenv("PREST_CONF", helpers.TestExposeConfigPath())
	config.Load()
	h := controllers.NewHandlersFromConfig(config.PrestConf)
	r := mux.NewRouter()
	r.HandleFunc("/tables", h.Catalog.ListTables).Methods("GET")
	r.HandleFunc("/databases", h.Catalog.ListDatabases).Methods("GET")
	r.HandleFunc("/schemas", h.Catalog.ListSchemas).Methods("GET")
	n := middlewares.GetApp()
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()
	resp, _ := http.Get(server.URL + "/tables")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp, _ = http.Get(server.URL + "/databases")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	resp, _ = http.Get(server.URL + "/schemas")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
