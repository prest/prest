package controllers

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/adapters/mockgen"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers/auth"
	"github.com/prest/prest/testutils"
)

var (
	defaultConfig = &config.Prest{
		AuthEnabled:  true,
		AuthEncrypt:  "MD5",
		AuthSchema:   "public",
		AuthTable:    "prest_users",
		AuthUsername: "username",
		AuthPassword: "password",
		Debug:        true,
	}

	authUser = auth.User{
		ID:       1,
		Name:     "prest-user",
		Username: "arxdsilva",
	}
)

// todo: fix these tests
func initAuthRoutes(enabled bool, c Config) *mux.Router {
	r := mux.NewRouter()
	if enabled {
		r.HandleFunc("/auth", c.Auth).Methods("POST")
	}
	return r
}

func Test_basicPasswordCheck(t *testing.T) {
	dc := *defaultConfig
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)

	ctrl2 := gomock.NewController(t)
	adapter2 := mockgen.NewMockScanner(ctrl2)

	adapter2.EXPECT().Err().Return(nil)
	adapter2.EXPECT().Scan(&auth.User{}).Return(1, nil)

	adapter.EXPECT().Query(
		"SELECT * FROM public.prest_users WHERE username=$1 AND password=$2 LIMIT 1", "test@postgres.rest", "e10adc3949ba59abbe56e057f20f883e").
		Return(adapter2)

	dc.Adapter = adapter

	cfg := New(&dc, nil)
	_, err := cfg.basicPasswordCheck(context.Background(), "test@postgres.rest", "123456")
	if err != nil {
		t.Errorf("expected authenticated user, got: %s", err)
	}
}

func Test_getSelectQuery(t *testing.T) {
	cfg := New(defaultConfig, nil)

	expected := "SELECT * FROM public.prest_users WHERE username=$1 AND password=$2 LIMIT 1"
	query := cfg.getSelectQuery()

	require.Equal(t, expected, query)
}

func Test_encrypt(t *testing.T) {
	cfg := New(&config.Prest{AuthEncrypt: "MD5"}, nil)

	pwd := "123456"
	enc := encrypt(cfg.server.AuthEncrypt, pwd)

	md5Enc := fmt.Sprintf("%x", md5.Sum([]byte(pwd)))
	if enc != md5Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, md5Enc)
	}

	cfg.server.AuthEncrypt = "SHA1"

	enc = encrypt(cfg.server.AuthEncrypt, pwd)

	sha1Enc := fmt.Sprintf("%x", sha1.Sum([]byte(pwd)))
	if enc != sha1Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, sha1Enc)
	}
}

func TestAuthDisable(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{
		server: &config.Prest{
			AuthEnabled: false,
			Debug:       true,
		},
		adapter: adapter}

	server := httptest.NewServer(initAuthRoutes(false, h))
	defer server.Close()

	t.Log("/auth request POST method, disable auth")
	testutils.DoRequest(t, server.URL+"/auth", nil, "POST", http.StatusNotFound, "AuthDisable")
}

func TestAuthEnable(t *testing.T) {

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int

		wantAuth bool
		authType string
	}{
		{"/auth request GET method", "/auth", "GET", http.StatusMethodNotAllowed, false, ""},
		{"/auth request POST method basic auth", "/auth", "POST", http.StatusBadRequest, false, "basic"},
		{"/auth request POST method no auth provided", "/auth", "POST", http.StatusUnauthorized, true, ""},
	}

	for _, tc := range testCases {
		t.Log(tc.description)

		ctrl := gomock.NewController(t)
		adapter := mockgen.NewMockAdapter(ctrl)

		if tc.wantAuth {
			ctrl2 := gomock.NewController(t)
			adapter2 := mockgen.NewMockScanner(ctrl2)

			adapter.EXPECT().QueryCtx(gomock.Any(), "SELECT * FROM . WHERE =$1 AND =$2 LIMIT 1",
				gomock.Any(), gomock.Any()).Return(adapter2)

			adapter2.EXPECT().Err().Return(nil)
			adapter2.EXPECT().Scan(&auth.User{}).Return(0, nil)
		}

		h := Config{
			server: &config.Prest{
				Debug:       true,
				AuthEnabled: true,
				AuthType:    tc.authType,
			},
			adapter: adapter,
		}

		server := httptest.NewServer(initAuthRoutes(true, h))

		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "AuthEnable")

		server.Close()
	}
}
