package middlewares_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/middlewares"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
)

func TestMain(m *testing.M) {
	helpers.EnsureTestConfigEnv()
	os.Exit(m.Run())
}

func TestInitApp(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	require.NotNil(t, middlewares.New(cfg))
}

func TestGetApp(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	require.NotNil(t, middlewares.New(cfg))
}

func TestGetAppWithReorderedMiddleware(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	n := middlewares.NewForTest(cfg, negroni.HandlerFunc(customMiddleware))
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()

	// GET / through custom middleware prepended to the stack.
	// Expected to succeed with HTTP status OK and include the custom middleware JSON message.
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
	cfg := helpers.LoadTestConfig(t)
	n := middlewares.New(cfg)
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()

	// GET / with the default middleware stack (no custom reorder).
	// Expected to succeed and set a JSON Content-Type.
	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

func Test_Middleware_DoesntBlock_CustomRoutes(t *testing.T) {
	t.Setenv("PREST_DEBUG", "true")
	cfg := helpers.LoadTestConfig(t)
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	h := controllers.NewHandlersFromConfig(cfg)
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", h.CRUD.Select).Methods("GET")

	r.PathPrefix("/").Handler(negroni.New(
		middlewares.AccessControl(cfg.Adapter),
		negroni.Wrap(crudRoutes),
	))
	n := middlewares.New(cfg)
	n.UseHandler(r)

	server := httptest.NewServer(n)
	defer server.Close()

	// Hit a custom application route registered before CRUD access control.
	// Expected to succeed and return the custom route body.
	resp, err := http.Get(server.URL)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	defer resp.Body.Close()

	require.Contains(t, string(body), "custom route")
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")

	// CRUD path still enforces AccessControl for restricted tables.
	// Expected to fail with HTTP status Unauthorized and an authorization message.
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
	nd := appTest(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	// GET / with PREST_DEBUG=true.
	// Expected to succeed with HTTP status OK.
	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestEnableDefaultJWT(t *testing.T) {
	t.Setenv("PREST_JWT_DEFAULT", "false")
	t.Setenv("PREST_DEBUG", "false")
	nd := appTest(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	// GET / with JWT default disabled and debug off.
	// Expected to fail with HTTP status NotImplemented from the test stub handler.
	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotImplemented, resp.StatusCode)
}

func TestJWTIsRequired(t *testing.T) {
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	nd := appTestWithJwt(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	// GET / with JWT required and no Authorization header.
	// Expected to fail with HTTP status Unauthorized.
	resp, err := http.Get(serverd.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJWTSignatureOk(t *testing.T) {
	bearer := "Bearer eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJpYXQiOjE1MTc1NjM2MTYsImlzcyI6InByaXZhdGUiLCJqdGkiOiJjZWZhNzRmZS04OTRjLWZmNjMtZDgxNi00NjIwYjhjZDkyZWUiLCJvcmciOiJwcml2YXRlIiwic3ViIjoiam9obi5kb2UifQ.zLWkEd4hP4XdCD_DlRy6mgPeKwEl1dcdtx5A_jHSfmc87EsrGgNSdi8eBTzCgSU0jgV6ssTgQwzY6x4egze2xA"
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS512")
	nd := appTestWithJwt(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	req.Header.Add("authorization", bearer)

	// GET / with a valid HS512 JWT.
	// Expected to succeed with HTTP status OK.
	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respd.StatusCode)
}

func TestJWTSignatureKo(t *testing.T) {
	bearer := "Bearer: eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImpvaG4uZG9lQHNvbWV3aGVyZS5jb20iLCJleHAiOjE1MjUzMzk2MTYsImlhdCI6MTUxNzU2MzYxNiwiaXNzIjoicHJpdmF0ZSIsImp0aSI6ImNlZmE3NGZlLTg5NGMtZmY2My1kODE2LTQ2MjBiOGNkOTJlZSIsIm9yZyI6InByaXZhdGUiLCJzdWIiOiJqb2huLmRvZSJ9.zGP1Xths2bK2r9FN0Gv1SzyoisO0dhRwvqrPvunGxUyU5TbkfdnTcQRJNYZzJfGILeQ9r3tbuakWm-NIoDlbbA"
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS256")
	nd := appTestWithJwt(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	req.Header.Add("authorization", bearer)

	// GET / with a malformed/invalid bearer token under HS256.
	// Expected to fail with HTTP status Unauthorized.
	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, respd.StatusCode)
}

func appTest(t *testing.T) *negroni.Negroni {
	t.Helper()
	cfg, err := config.Load()
	require.NoError(t, err)
	n := middlewares.New(cfg)
	r := mux.NewRouter()
	if !cfg.Debug && !cfg.EnableDefaultJWT {
		n.UseHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
		return n
	}
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}

func appTestWithJwt(t *testing.T) *negroni.Negroni {
	t.Helper()
	cfg, err := config.Load()
	require.NoError(t, err)
	n := middlewares.New(cfg)
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")

	n.UseHandler(r)
	return n
}

func Test_CORS_Middleware(t *testing.T) {
	t.Setenv("PREST_DEBUG", "true")
	t.Setenv("PREST_CORS_ALLOWORIGIN", "*")
	t.Setenv("PREST_CONF", helpers.TestConfigPath())
	cfg, err := config.Load()
	require.NoError(t, err)
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("custom route")) })
	n := middlewares.New(cfg)
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()

	// OPTIONS preflight with Access-Control-Request-Method GET.
	// Expected to succeed with HTTP status NoContent and an empty body.
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
	t.Setenv("PREST_DEBUG", "true")
	t.Setenv("PREST_CONF", helpers.TestExposeConfigPath())
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NoError(t, app.EnsureAdapter(cfg))
	h := controllers.NewHandlersFromConfig(cfg)
	r := mux.NewRouter()
	r.HandleFunc("/tables", h.Catalog.ListTables).Methods("GET")
	r.HandleFunc("/databases", h.Catalog.ListDatabases).Methods("GET")
	r.HandleFunc("/schemas", h.Catalog.ListSchemas).Methods("GET")
	n := middlewares.New(cfg)
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()

	// Catalog /tables with expose-tables restricting unauthenticated access.
	// Expected to fail with HTTP status Unauthorized.
	resp, _ := http.Get(server.URL + "/tables")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Catalog /databases under the same restriction.
	// Expected to fail with HTTP status Unauthorized.
	resp, _ = http.Get(server.URL + "/databases")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Catalog /schemas under the same restriction.
	// Expected to fail with HTTP status Unauthorized.
	resp, _ = http.Get(server.URL + "/schemas")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
