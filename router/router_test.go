package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/adapters/mockgen"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers/auth"
	"github.com/prest/prest/testutils"
)

func TestRoutes(t *testing.T) {
	require.NotNil(t, New(&config.Prest{}))
}

// todo: split this into mini tests
func Test_DefaultRouters(t *testing.T) {

	var testCases = []struct {
		url    string
		method string
		status int

		wantGetDBs bool
	}{
		{
			url:        "/databases",
			method:     "GET",
			status:     http.StatusOK,
			wantGetDBs: true,
		},
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
	}
	for _, tc := range testCases {
		ctrl := gomock.NewController(t)
		adapter := mockgen.NewMockAdapter(ctrl)

		ctrl2 := gomock.NewController(t)
		adapter2 := mockgen.NewMockScanner(ctrl2)

		adapter.EXPECT().Query(" ").Return(adapter2)

		adapter2.EXPECT().Err().Return(nil)
		adapter2.EXPECT().Bytes().Return([]byte("[]"))

		adapter.EXPECT().WhereByRequest(gomock.Any(), gomock.Any()).Return("", nil, nil)
		adapter.EXPECT().DatabaseWhere(gomock.Any()).Return("")
		adapter.EXPECT().DatabaseClause(gomock.Any()).Return("", false)
		adapter.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
		adapter.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
		adapter.EXPECT().DatabaseOrderBy(gomock.Any(), gomock.Any()).Return("")
		adapter.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

		cfg := &config.Prest{
			Adapter: adapter,
		}
		server := httptest.NewServer(New(cfg).router)

		t.Log(tc.method, "\t", tc.url)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, tc.url)
		server.Close()
	}
}

func Test_Route_Databases(t *testing.T) {
	var testCases = []struct {
		url    string
		method string
		status int
	}{
		{
			url:    "/databases",
			method: "GET",
			status: http.StatusOK,
		},
	}
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)

	ctrl2 := gomock.NewController(t)
	adapter2 := mockgen.NewMockScanner(ctrl2)

	adapter.EXPECT().Query(" ").Return(adapter2)

	adapter2.EXPECT().Err().Return(nil)
	adapter2.EXPECT().Bytes().Return([]byte("[]"))

	adapter.EXPECT().WhereByRequest(gomock.Any(), gomock.Any()).Return("", nil, nil)
	adapter.EXPECT().DatabaseWhere(gomock.Any()).Return("")
	adapter.EXPECT().DatabaseClause(gomock.Any()).Return("", false)
	adapter.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	adapter.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	adapter.EXPECT().DatabaseOrderBy(gomock.Any(), gomock.Any()).Return("")
	adapter.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	cfg := &config.Prest{
		Adapter: adapter,
	}
	server := httptest.NewServer(New(cfg).router)

	for _, tc := range testCases {
		t.Log(tc.method, "\t", tc.url)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, tc.url)
		server.Close()
	}
}

func Test_AuthRouteActive_NotFound(t *testing.T) {
	cfg := &config.Prest{
		Debug:       true,
		AuthEnabled: true,
		// bypass middleware checks, only verify route access
		JWTWhiteList: []string{"/auth"},
	}

	server := httptest.NewServer(New(cfg).router)
	testutils.DoRequest(t, server.URL+"/auth", nil, "GET", http.StatusNotFound, "AuthEnable")
}

func Test_AuthRouteActive_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)

	ctrl2 := gomock.NewController(t)
	adapter2 := mockgen.NewMockScanner(ctrl2)

	adapter.EXPECT().Query("SELECT * FROM . WHERE =$1 AND =$2 LIMIT 1",
		gomock.Any(), gomock.Any()).Return(adapter2)

	adapter2.EXPECT().Err().Return(nil)
	adapter2.EXPECT().Scan(&auth.User{}).Return(0, nil)

	cfg := &config.Prest{
		Debug:       true,
		AuthEnabled: true,
		Adapter:     adapter,
	}

	server := httptest.NewServer(New(cfg).router)
	testutils.DoRequest(t, server.URL+"/auth", nil, "POST", http.StatusUnauthorized, "AuthEnable")
}
