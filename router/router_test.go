package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/middlewares"
	"github.com/prest/prest/v2/plugins"
)

const testRouterJWTKey = "router-test-secret-key-at-least-32-bytes!"

// routerFixture holds the mux plus the mocks a test may set expectations on.
type routerFixture struct {
	mux     *mux.Router
	adapter *mockgen.MockAdapter
	builder *mockgen.MockRequestQueryBuilder
	db      *mockgen.MockDatabaseRegistry
	pinger  *mockgen.MockDatabasePinger
}

// newTestRouter wires RegisterRoutes exactly as app.go does, but with mock
// ports so no database is touched. Unauthenticated requests must be rejected
// before any handler runs, so the mocks carry no expectations by default.
func newTestRouter(t *testing.T, authEnabled bool) *routerFixture {
	t.Helper()
	return newTestRouterWithRegistry(t, authEnabled, false)
}

// newTestRouterWithRegistry also enables telemetry route tagging and the
// database-backed query registry (adminStack) when withRegistry is true.
func newTestRouterWithRegistry(t *testing.T, authEnabled, withRegistry bool) *routerFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &routerFixture{
		adapter: mockgen.NewMockAdapter(ctrl),
		builder: mockgen.NewMockRequestQueryBuilder(ctrl),
		db:      mockgen.NewMockDatabaseRegistry(ctrl),
		pinger:  mockgen.NewMockDatabasePinger(ctrl),
	}
	cfg := &config.Prest{
		AuthEnabled: authEnabled,
		JWTKey:      testRouterJWTKey,
		JWTAlgo:     "HS256",
		PluginPath:  t.TempDir(),
		Adapter:     f.adapter,
	}
	if withRegistry {
		cfg.Otel.Enabled = true
		cfg.QueriesConf.RegisterEnabled = true
		cfg.QueriesConf.Storage = config.QueriesStorageDatabase
	}
	deps := controllers.Deps{
		Catalog:     mockgen.NewMockCatalogQuerier(ctrl),
		Builder:     f.builder,
		Executor:    mockgen.NewMockQueryExecutor(ctrl),
		SQL:         mockgen.NewMockSQLBuilder(ctrl),
		Perms:       f.adapter,
		Scripts:     mockgen.NewMockScriptRunner(ctrl),
		DB:          f.db,
		Pinger:      f.pinger,
		Readiness:   mockgen.NewMockReadinessChecker(ctrl),
		AdapterName: "postgres",
		PGDatabase:  "prest-test",
	}
	var adminStack *middlewares.AdminQueryStack
	if withRegistry {
		deps.QueryRegistry = mockgen.NewMockQueryRegistry(ctrl)
		adminStack = middlewares.NewAdminQueryStack(cfg)
	}
	h := controllers.NewHandlers(deps, cfg)
	plg := plugins.New(cfg)
	crudStack := middlewares.NewCRUDStack(cfg, plg)
	queryStack := middlewares.NewQueryStack(cfg, middlewares.ScriptPermsFromAdapter(cfg.Adapter))

	f.mux = mux.NewRouter().StrictSlash(true)
	RegisterRoutes(f.mux, cfg, h, crudStack, queryStack, adminStack, plg)
	return f
}

func signRouterTestJWT(t *testing.T, username string) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: []byte(testRouterJWTKey)},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)
	now := time.Now()
	token, err := jwt.Signed(sig).Claims(auth.Claims{
		UserInfo:  auth.User{Username: username},
		NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(now.Add(time.Hour)),
	}).Serialize()
	require.NoError(t, err)
	return token
}

func serve(f *routerFixture, method, target, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

// With auth.enabled every data/metadata route must reject a request that
// carries no token (GHSA-fhf5-m388-4w4p catalog bypass, GHSA-fmr6-9j48-xrqp
// unauthenticated /_PLUGIN).
func TestRegisterRoutes_AuthEnabled_RejectsMissingToken(t *testing.T) {
	cases := []struct {
		description string
		method      string
		target      string
		skipWindows bool
	}{
		{description: "list databases without token -> 401", method: http.MethodGet, target: "/databases"},
		{description: "list schemas without token -> 401", method: http.MethodGet, target: "/schemas"},
		{description: "list tables without token -> 401", method: http.MethodGet, target: "/tables"},
		{description: "list tables of database/schema without token -> 401", method: http.MethodGet, target: "/prest-test/public"},
		{description: "show table structure without token -> 401", method: http.MethodGet, target: "/show/prest-test/public/test"},
		{description: "plugin handler without token -> 401", method: http.MethodGet, target: "/_PLUGIN/hello/Hello", skipWindows: true},
		{description: "MCP POST without token -> 401", method: http.MethodPost, target: "/_mcp"},
		{description: "MCP GET without token -> 401", method: http.MethodGet, target: "/_mcp"},
		{description: "CRUD select without token -> 401", method: http.MethodGet, target: "/prest-test/public/test"},
		{description: "CRUD insert without token -> 401", method: http.MethodPost, target: "/prest-test/public/test"},
		{description: "batch insert without token -> 401", method: http.MethodPost, target: "/batch/prest-test/public/test"},
		{description: "custom query script without token -> 401", method: http.MethodGet, target: "/_QUERIES/queries/get_all"},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			if tc.skipWindows && runtime.GOOS == "windows" {
				t.Skip("/_PLUGIN is not registered on windows")
			}
			f := newTestRouter(t, true)
			rec := serve(f, tc.method, tc.target, "")
			require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
		})
	}
}

// Liveness probes stay public even when auth is enabled.
func TestRegisterRoutes_AuthEnabled_HealthIsPublic(t *testing.T) {
	f := newTestRouter(t, true)
	f.pinger.EXPECT().Ping(gomock.Any()).Return(nil)

	rec := serve(f, http.MethodGet, "/_health", "")
	require.NotEqual(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
}

// A valid token lets catalog listings through the stack to the handler; the
// mocked builder fails, so reaching the handler shows up as 400, not 401.
func TestRegisterRoutes_AuthEnabled_ValidTokenReachesCatalogHandler(t *testing.T) {
	f := newTestRouter(t, true)
	f.builder.EXPECT().WhereByRequest(gomock.Any(), gomock.Any()).
		Return("", nil, errors.New("builder stub")).Times(3)
	token := signRouterTestJWT(t, "alice")

	for _, target := range []string{"/databases", "/schemas", "/tables"} {
		rec := serve(f, http.MethodGet, target, token)
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s body: %s", target, rec.Body.String())
	}
}

// /show/{db}/{schema}/{table} applies the table ACL: an authenticated user
// without read permission is refused before the handler runs.
func TestRegisterRoutes_AuthEnabled_ShowAppliesTableACL(t *testing.T) {
	token := signRouterTestJWT(t, "alice")

	t.Run("denied by ACL -> 401", func(t *testing.T) {
		f := newTestRouter(t, true)
		f.adapter.EXPECT().
			TablePermissions("prest-test", "public", "test", "read", "alice").
			Return(false)
		rec := serve(f, http.MethodGet, "/show/prest-test/public/test", token)
		require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("allowed by ACL -> reaches handler", func(t *testing.T) {
		f := newTestRouter(t, true)
		f.adapter.EXPECT().
			TablePermissions("prest-test", "public", "test", "read", "alice").
			Return(true)
		// Unregistered database makes the handler answer 400, proving it ran.
		f.db.EXPECT().IsRegistered("prest-test").Return(false)
		rec := serve(f, http.MethodGet, "/show/prest-test/public/test", token)
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
	})
}

// With auth disabled the catalog routes keep working without a token: each
// reaches its handler (which fails on the stubbed ports with 400).
func TestRegisterRoutes_AuthDisabled_CatalogRoutesOpen(t *testing.T) {
	cases := []struct {
		description string
		target      string
		setup       func(f *routerFixture)
	}{
		{
			description: "list databases reaches handler -> 400 from builder stub",
			target:      "/databases",
			setup: func(f *routerFixture) {
				f.builder.EXPECT().WhereByRequest(gomock.Any(), gomock.Any()).Return("", nil, errors.New("stub"))
			},
		},
		{
			description: "list schemas reaches handler -> 400 from builder stub",
			target:      "/schemas",
			setup: func(f *routerFixture) {
				f.builder.EXPECT().WhereByRequest(gomock.Any(), gomock.Any()).Return("", nil, errors.New("stub"))
			},
		},
		{
			description: "list tables reaches handler -> 400 from builder stub",
			target:      "/tables",
			setup: func(f *routerFixture) {
				f.builder.EXPECT().WhereByRequest(gomock.Any(), gomock.Any()).Return("", nil, errors.New("stub"))
			},
		},
		{
			description: "list tables by database/schema reaches handler -> 400 unregistered db",
			target:      "/prest-test/public",
			setup: func(f *routerFixture) {
				f.db.EXPECT().IsRegistered("prest-test").Return(false)
			},
		},
		{
			description: "show table passes ACL (anonymous) and reaches handler -> 400 unregistered db",
			target:      "/show/prest-test/public/test",
			setup: func(f *routerFixture) {
				f.adapter.EXPECT().TablePermissions("prest-test", "public", "test", "read", "").Return(true)
				f.db.EXPECT().IsRegistered("prest-test").Return(false)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			f := newTestRouter(t, false)
			tc.setup(f)
			rec := serve(f, http.MethodGet, tc.target, "")
			require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		})
	}
}

// Query-registry admin routes (registered only with database storage) sit
// behind adminRoute and reject requests without a token.
func TestRegisterRoutes_AuthEnabled_QueryRegistryRejectsMissingToken(t *testing.T) {
	cases := []struct {
		description string
		method      string
		target      string
	}{
		{description: "list registry without token -> 401", method: http.MethodGet, target: "/_QUERIES/registry"},
		{description: "create registry entry without token -> 401", method: http.MethodPost, target: "/_QUERIES/registry"},
		{description: "list registry for database without token -> 401", method: http.MethodGet, target: "/_QUERIES/registry/prest-test"},
		{description: "get registry entry without token -> 401", method: http.MethodGet, target: "/_QUERIES/registry/loc/name"},
		{description: "update registry entry without token -> 401", method: http.MethodPut, target: "/_QUERIES/registry/loc/name"},
		{description: "delete registry entry without token -> 401", method: http.MethodDelete, target: "/_QUERIES/registry/prest-test/loc/name"},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			f := newTestRouterWithRegistry(t, true, true)
			rec := serve(f, tc.method, tc.target, "")
			require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
		})
	}
}
