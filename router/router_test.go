package router

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	srvMock "github.com/prest/prest/controllers/mockgen"

	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers"
	"github.com/prest/prest/middlewares"
	"github.com/prest/prest/plugins"
	"github.com/prest/prest/testutils"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
)

func TestRoutes(t *testing.T) {
	cfg, err := New(&config.Prest{}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

// todo: split this into mini tests
// only verify route access
// remove all adapter/handler necessity
// move handler functionality test to controllers pkg
func Test_DefaultRouters(t *testing.T) {
	// var testCases = []struct {
	// 	url    string
	// 	method string
	// 	status int

	// 	wantGetDBs bool
	// }{
	// 	{
	// 		url:        "/databases",
	// 		method:     "GET",
	// 		status:     http.StatusOK,
	// 		wantGetDBs: true,
	// 	},
	// {
	// 	url:        "/schemas",
	// 	method:     "GET",
	// 	status:     http.StatusOK,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/_QUERIES/{queriesLocation}/{script}",
	// 	method:     "GET",
	// 	status:     http.StatusBadRequest,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/{database}/{schema}",
	// 	method:     "GET",
	// 	status:     http.StatusBadRequest,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/show/{database}/{schema}/{table}",
	// 	method:     "GET",
	// 	status:     http.StatusBadRequest,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/{database}/{schema}/{table}",
	// 	method:     "GET",
	// 	status:     http.StatusUnauthorized,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/{database}/{schema}/{table}",
	// 	method:     "POST",
	// 	status:     http.StatusUnauthorized,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/batch/{database}/{schema}/{table}",
	// 	method:     "POST",
	// 	status:     http.StatusBadRequest,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/{database}/{schema}/{table}",
	// 	method:     "DELETE",
	// 	status:     http.StatusUnauthorized,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/{database}/{schema}/{table}",
	// 	method:     "PUT",
	// 	status:     http.StatusUnauthorized,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/{database}/{schema}/{table}",
	// 	method:     "PATCH",
	// 	status:     http.StatusUnauthorized,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/auth",
	// 	method:     "GET",
	// 	status:     http.StatusNotFound,
	// 	wantGetDBs: false,
	// },
	// {
	// 	url:        "/",
	// 	method:     "GET",
	// 	status:     http.StatusNotFound,
	// 	wantGetDBs: false,
	// },
	// }
	// for _, tc := range testCases {
	// 		ctrl := gomock.NewController(t)
	// 		adapter := mockgen.NewMockAdapter(ctrl)

	// 		ctrl2 := gomock.NewController(t)
	// 		adapter2 := mockgen.NewMockScanner(ctrl2)

	// 		adapter.EXPECT().Query(" ").Return(adapter2)

	// 		adapter2.EXPECT().Err().Return(nil)
	// 		adapter2.EXPECT().Bytes().Return([]byte("[]"))

	// 		adapter.EXPECT().WhereByRequest(gomock.Any(), gomock.Any()).Return("", nil, nil)
	// 		adapter.EXPECT().DatabaseWhere(gomock.Any()).Return("")
	// 		adapter.EXPECT().DatabaseClause(gomock.Any()).Return("", false)
	// 		adapter.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	// 		adapter.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	// 		adapter.EXPECT().DatabaseOrderBy(gomock.Any(), gomock.Any()).Return("")
	// 		adapter.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	// 		cfg := &Config{srvCfg: &config.Prest{}}
	// 		server := httptest.NewServer(cfg.router)

	// 		t.Log(tc.method, "\t", tc.url)
	// 		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, tc.url)
	// 		server.Close()
	// 	}
	// }

	// func Test_Route_Databases(t *testing.T) {
	// 	var testCases = []struct {
	// 		url    string
	// 		method string
	// 		status int
	// 	}{
	// 		{
	// 			url:    "/databases",
	// 			method: "GET",
	// 			status: http.StatusOK,
	// 		},
	// 	}
	// 	ctrl := gomock.NewController(t)
	// 	adapter := mockgen.NewMockAdapter(ctrl)

	// 	ctrl2 := gomock.NewController(t)
	// 	adapter2 := mockgen.NewMockScanner(ctrl2)

	// 	adapter.EXPECT().Query(" ").Return(adapter2)

	// 	adapter2.EXPECT().Err().Return(nil)
	// 	adapter2.EXPECT().Bytes().Return([]byte("[]"))

	// 	adapter.EXPECT().WhereByRequest(gomock.Any(), gomock.Any()).Return("", nil, nil)
	// 	adapter.EXPECT().DatabaseWhere(gomock.Any()).Return("")
	// 	adapter.EXPECT().DatabaseClause(gomock.Any()).Return("", false)
	// 	adapter.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	// 	adapter.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	// 	adapter.EXPECT().DatabaseOrderBy(gomock.Any(), gomock.Any()).Return("")
	// 	adapter.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	// 	cfg := &Config{srvCfg: &config.Prest{}}
	// 	cfg.router = mux.NewRouter().StrictSlash(true)
	// 	server := httptest.NewServer(cfg.router)

	//	for _, tc := range testCases {
	//		t.Log(tc.method, "\t", tc.url)
	//		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, tc.url)
	//		server.Close()
	//	}
}

func Test_AuthRouteActive_NotFound(t *testing.T) {
	// cfg := &config.Prest{
	// 	Debug:       true,
	// 	AuthEnabled: true,
	// 	// bypass middleware checks, only verify route access
	// 	JWTWhiteList: []string{"/auth"},
	// }

	// server := httptest.NewServer(New(cfg).router)
	// testutils.DoRequest(t, server.URL+"/auth", nil, "GET", http.StatusNotFound, "AuthEnable")
}

func Test_AuthRouteActive_Unauthorized(t *testing.T) {
	// ctrl := gomock.NewController(t)
	// adapter := mockgen.NewMockAdapter(ctrl)

	// ctrl2 := gomock.NewController(t)
	// adapter2 := mockgen.NewMockScanner(ctrl2)

	// adapter.EXPECT().QueryCtx(gomock.Any(), "SELECT * FROM . WHERE =$1 AND =$2 LIMIT 1",
	// 	gomock.Any(), gomock.Any()).Return(adapter2)

	// adapter2.EXPECT().Err().Return(nil)
	// adapter2.EXPECT().Scan(&auth.User{}).Return(0, nil)

	// server := httptest.NewServer(.router)
	// testutils.DoRequest(t, server.URL+"/auth", nil, "POST", http.StatusUnauthorized, "AuthEnable")
}

func Test_ConfigRoutes(t *testing.T) {
	t.Parallel()

	r := &Config{
		srvCfg: &config.Prest{
			AuthEnabled:  false,
			JWTKey:       "jwt-key",
			JWTWhiteList: []string{"/auth"},
			ExposeConf:   config.ExposeConf{Enabled: true},
			PluginPath:   "/path/to/plugins",
		},
		router: mux.NewRouter().StrictSlash(true),
		cache:  nil, // provide cache implementation if needed
	}

	var testCases = []struct {
		url    string
		method string
		status int

		wantGetDBs bool
	}{
		{},
	}
	_ = testCases

	ctrl := gomock.NewController(t)
	srv := srvMock.NewMockServer(ctrl)

	srv.EXPECT().Auth(gomock.Any(), gomock.Any()).AnyTimes().Return()

	err := r.ConfigRoutes(srv)
	require.NoError(t, err)

	// srv

	// Test /auth route
	authRoute := "/auth"
	authHandler := srv.Auth
	testutils.DoRequest(t, r.router, authRoute, nil, "POST", http.StatusOK, authHandler)

	// Test /databases route
	databasesRoute := "/databases"
	getDatabasesHandler := srv.GetDatabases
	testutils.DoRequest(t, r.router, databasesRoute, nil, "GET", http.StatusOK, getDatabasesHandler)

	// Test /schemas route
	schemasRoute := "/schemas"
	getSchemasHandler := srv.GetSchemas
	testutils.DoRequest(t, r.router, schemasRoute, nil, "GET", http.StatusOK, getSchemasHandler)

	// Test /tables route
	tablesRoute := "/tables"
	getTablesHandler := srv.GetTables
	testutils.DoRequest(t, r.router, tablesRoute, nil, "GET", http.StatusOK, getTablesHandler)

	// Test /_QUERIES/{queriesLocation}/{script} route
	queriesRoute := "/_QUERIES/{queriesLocation}/{script}"
	executeFromScriptsHandler := srv.ExecuteFromScripts
	testutils.DoRequest(t, r.router, queriesRoute, nil, "GET", http.StatusOK, executeFromScriptsHandler)

	// Test /{database}/{schema} route
	databaseSchemaRoute := "/{database}/{schema}"
	getTablesByDatabaseAndSchemaHandler := srv.GetTablesByDatabaseAndSchema
	testutils.DoRequest(t, r.router, databaseSchemaRoute, nil, "GET", http.StatusOK, getTablesByDatabaseAndSchemaHandler)

	// Test /show/{database}/{schema}/{table} route
	showTableRoute := "/show/{database}/{schema}/{table}"
	showTableHandler := srv.ShowTable
	testutils.DoRequest(t, r.router, showTableRoute, nil, "GET", http.StatusOK, showTableHandler)

	// Test CRUD routes
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", srv.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", srv.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", srv.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", srv.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", srv.UpdateTable).Methods("PUT", "PATCH")

	// Test /_health route
	healthRoute := "/_health"
	wrappedHealthCheckHandler := srv.WrappedHealthCheck(controllers.DefaultCheckList)
	testutils.DoRequest(t, r.router, healthRoute, nil, "GET", http.StatusOK, wrappedHealthCheckHandler)

	// Test middleware stack
	r.router.PathPrefix("/").Handler(
		negroni.New(
			middlewares.ExposureMiddleware(&r.srvCfg.ExposeConf),
			middlewares.AccessControl(srv.GetAdapter().TablePermissions),
			middlewares.AuthMiddleware(
				r.srvCfg.AuthEnabled, r.srvCfg.JWTKey, r.srvCfg.JWTWhiteList),
			middlewares.CacheMiddleware(r.srvCfg, r.cache),
			plugins.MiddlewarePlugin(r.srvCfg.PluginPath, r.srvCfg.PluginMiddlewareList),
			negroni.Wrap(crudRoutes),
		),
	)

	// Test the middleware stack with a request
	server := httptest.NewServer(r.router)
	testutils.DoRequest(t, server.URL+"/test", nil, "GET", http.StatusOK, nil)
	server.Close()

	// Test the plugin route on non-Windows systems
	if runtime.GOOS != "windows" {
		pluginRoute := "/_PLUGIN/{file}/{func}"
		pluginHandler := srv.Plugin
		testutils.DoRequest(t, r.router, pluginRoute, nil, "GET", http.StatusOK, pluginHandler)
	}
}
