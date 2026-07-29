package middlewares_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
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
	t.Setenv("PREST_JWT_KEY", "test-jwt-hmac-secret-key-32bytes")
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
	const jwtKey = "integration-test-secret-key-32b!!"

	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: []byte(jwtKey)},
		(&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)
	bearer, err := jwt.Signed(sig).Claims(auth.Claims{
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}).Serialize()
	require.NoError(t, err)

	// GET a protected table through the deployed auth-enabled PostgreSQL service.
	// Expected to succeed because the HS256 token uses the configured key.
	testutils.DoRequestWithHeaders(
		t,
		helpers.AuthServerURL(t)+"/prest-test/public/test",
		nil,
		http.MethodGet,
		http.StatusOK,
		"valid HS256 token reaches protected table",
		map[string]string{"Authorization": "Bearer " + bearer},
	)
}

func TestJWTSignatureKo(t *testing.T) {
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "test-jwt-hmac-secret-key-32bytes")
	t.Setenv("PREST_JWT_ALGO", "HS256")
	nd := appTestWithJwt(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	const differentHS512Key = "test-jwt-hmac-secret-key-64bytes-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS512, Key: []byte(differentHS512Key)},
		(&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)
	bearer, err := jwt.Signed(sig).Claims(auth.Claims{
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}).Serialize()
	require.NoError(t, err)

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	req.Header.Add("authorization", "Bearer "+bearer)

	// GET / with a valid HS512 token while only HS256 is configured.
	// Expected to fail because the algorithm allowlist rejects HS512.
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

// TestExposeMCPEndpoint guards the /_mcp bypass of [expose]: the endpoint is
// not one of the three literal prefixes ExposureMiddleware gates
// (/databases, /tables, /schemas), so an operator who set
// expose.databases/schemas/tables = false to hide catalog discovery still had
// the full database, schema, table and column catalog served through /_mcp —
// via the JSON-RPC list tools and, with no call at all, via the GET discovery
// payload whose per-table tool names and descriptions embed those names.
// The MCP catalog tools now consult the same [expose] predicates as the REST
// routes.
func TestExposeMCPEndpoint(t *testing.T) {
	t.Setenv("PREST_DEBUG", "true")
	t.Setenv("PREST_CONF", helpers.TestExposeConfigPath())
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NoError(t, app.EnsureAdapter(cfg))

	h := controllers.NewHandlersFromConfig(cfg)
	r := mux.NewRouter()
	r.Handle("/_mcp", h.MCP.Handler()).Methods("GET", "POST")
	n := middlewares.New(cfg)
	n.UseHandler(r)
	server := httptest.NewServer(n)
	defer server.Close()

	// Baseline: the REST catalog route is denied, which is the operator intent
	// /_mcp must not contradict.
	resp, err := http.Get(server.URL + "/tables")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// GET /_mcp discovery must not enumerate the catalog it was set to hide:
	// no per-table select tools and no listing tools advertised.
	resp, err = http.Get(server.URL + "/_mcp")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	discovery := string(body)
	require.NotContains(t, discovery, "prest.select.")
	require.NotContains(t, discovery, "prest.list_databases")
	require.NotContains(t, discovery, "prest.list_schemas")
	require.NotContains(t, discovery, "prest.list_tables")

	// Each catalog tool is refused with the same "unauthorized listing" message
	// ExposureMiddleware returns for the REST routes.
	for _, tool := range []string{"prest.list_databases", "prest.list_schemas", "prest.list_tables"} {
		t.Log("tools/call " + tool + " must be denied while [expose] hides the catalog")
		call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `"}}`
		resp, err := http.Post(server.URL+"/_mcp", "application/json", strings.NewReader(call))
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		require.Contains(t, string(body), "unauthorized listing")
	}
}
