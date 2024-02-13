package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"

	adptMock "github.com/prest/prest/adapters/mockgen"
	"github.com/prest/prest/config"
	srvMock "github.com/prest/prest/controllers/mockgen"
)

var (
	cfg = Config{
		srvCfg: &config.Prest{
			AuthEnabled:  true,
			JWTKey:       "jwt-key",
			JWTWhiteList: []string{"/auth"},
			ExposeConf:   config.ExposeConf{Enabled: true},
			PluginPath:   "/path/to/plugins",
			Cache:        config.CacheConf{Enabled: false},
		},
		router: mux.NewRouter().StrictSlash(true),
		cache:  nil, // provide cache implementation if needed
	}
)

func TestRoutes(t *testing.T) {
	cfg, err := New(&config.Prest{}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func Test_ConfigRoutes_auth(t *testing.T) {
	t.Parallel()

	r := cfg

	ctrl := gomock.NewController(t)
	srv := srvMock.NewMockServer(ctrl)

	srv.EXPECT().WrappedHealthCheck(gomock.Any()).AnyTimes().Do(
		func(check interface{}) {})

	ma := adptMock.NewMockAdapter(ctrl)

	srv.EXPECT().GetAdapter().AnyTimes().Return(ma)

	srv.EXPECT().Auth(gomock.Any(), gomock.Any()).AnyTimes().Do(
		func(w, r interface{}) {})

	err := r.ConfigRoutes(srv)
	require.NoError(t, err)

	nr := negroni.New()
	nr.UseHandler(r.router)

	testSrv := httptest.NewServer(nr)
	defer testSrv.Close()

	resp, err := http.Post(testSrv.URL+"/auth", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func Test_ConfigRoutes_auth_notFound(t *testing.T) {
	t.Parallel()

	r := cfg
	ctrl := gomock.NewController(t)
	srv := srvMock.NewMockServer(ctrl)

	srv.EXPECT().WrappedHealthCheck(gomock.Any()).AnyTimes().Do(
		func(check interface{}) {})

	ma := adptMock.NewMockAdapter(ctrl)

	srv.EXPECT().GetAdapter().AnyTimes().Return(ma)

	srv.EXPECT().Auth(gomock.Any(), gomock.Any()).AnyTimes().Do(
		func(w, r interface{}) {})

	err := r.ConfigRoutes(srv)
	require.NoError(t, err)

	nr := negroni.New()
	nr.UseHandler(r.router)

	testSrv := httptest.NewServer(nr)
	defer testSrv.Close()

	resp, err := http.Get(testSrv.URL + "/auth")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func Test_ConfigRoutes_databases(t *testing.T) {
	t.Parallel()

	r := cfg

	ctrl := gomock.NewController(t)
	srv := srvMock.NewMockServer(ctrl)

	srv.EXPECT().WrappedHealthCheck(gomock.Any()).AnyTimes().Do(
		func(check interface{}) {})

	ma := adptMock.NewMockAdapter(ctrl)

	srv.EXPECT().GetAdapter().AnyTimes().Return(ma)

	srv.EXPECT().GetDatabases(gomock.Any(), gomock.Any()).AnyTimes().Do(
		func(w, r interface{}) {})

	err := r.ConfigRoutes(srv)
	require.NoError(t, err)

	nr := negroni.New()
	nr.UseHandler(r.router)

	testSrv := httptest.NewServer(nr)
	defer testSrv.Close()

	resp, err := http.Get(testSrv.URL + "/databases")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func Test_ConfigRoutes_databases_notFound(t *testing.T) {
	t.Parallel()

	r := cfg

	ctrl := gomock.NewController(t)
	srv := srvMock.NewMockServer(ctrl)

	srv.EXPECT().WrappedHealthCheck(gomock.Any()).AnyTimes().Do(
		func(check interface{}) {})

	ma := adptMock.NewMockAdapter(ctrl)

	srv.EXPECT().GetAdapter().AnyTimes().Return(ma)

	srv.EXPECT().GetDatabases(gomock.Any(), gomock.Any()).AnyTimes().Do(
		func(w, r interface{}) {})

	err := r.ConfigRoutes(srv)
	require.NoError(t, err)

	nr := negroni.New()
	nr.UseHandler(r.router)

	testSrv := httptest.NewServer(nr)
	defer testSrv.Close()

	// should be 404, is 401 now, need to debug it
	resp, err := http.Post(testSrv.URL+"/databases", "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func Test_ConfigRoutes_schemas(t *testing.T) {
	t.Parallel()

	r := cfg

	ctrl := gomock.NewController(t)
	srv := srvMock.NewMockServer(ctrl)

	srv.EXPECT().WrappedHealthCheck(gomock.Any()).AnyTimes().Do(
		func(check interface{}) {})

	ma := adptMock.NewMockAdapter(ctrl)

	srv.EXPECT().GetAdapter().AnyTimes().Return(ma)

	srv.EXPECT().GetSchemas(gomock.Any(), gomock.Any()).AnyTimes().Do(
		func(w, r interface{}) {})

	err := r.ConfigRoutes(srv)
	require.NoError(t, err)

	nr := negroni.New()
	nr.UseHandler(r.router)

	testSrv := httptest.NewServer(nr)
	defer testSrv.Close()

	resp, err := http.Get(testSrv.URL + "/schemas")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
