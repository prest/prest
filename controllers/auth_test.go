package controllers

import (
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
	"github.com/prest/prest/testutils"
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
	cfg := New(config.PrestConf, nil)

	_, err := cfg.basicPasswordCheck("test@postgres.rest", "123456")
	if err != nil {
		t.Errorf("expected authenticated user, got: %s", err)
	}
}

func Test_getSelectQuery(t *testing.T) {
	cfg := New(config.PrestConf, nil)

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

	config.PrestConf.AuthEncrypt = "SHA1"

	enc = encrypt(cfg.server.AuthEncrypt, pwd)

	sha1Enc := fmt.Sprintf("%x", sha1.Sum([]byte(pwd)))
	if enc != sha1Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, sha1Enc)
	}
}

func TestAuthDisable(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{adapter: adapter}

	server := httptest.NewServer(initAuthRoutes(false, h))
	defer server.Close()

	t.Log("/auth request POST method, disable auth")
	testutils.DoRequest(t, server.URL+"/auth", nil, "POST", http.StatusNotFound, "AuthDisable")
}

func TestAuthEnable(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{
		server:  &config.Prest{Debug: true},
		adapter: adapter,
	}

	config.PrestConf.AuthEnabled = true

	server := httptest.NewServer(initAuthRoutes(true, h))
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"/auth request GET method", "/auth", "GET", http.StatusMethodNotAllowed},
		{"/auth request POST method", "/auth", "POST", http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "AuthEnable")
	}
}
