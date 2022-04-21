package controllers

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/prest/prest/testutils"
)

func initAuthRoutes() *mux.Router {
	r := mux.NewRouter()
	// if auth is enabled
	if config.PrestConf.AuthEnabled {
		r.HandleFunc("/auth", Auth).Methods("POST")
	}
	return r
}

func Test_basicPasswordCheck(t *testing.T) {
	config.Load()
	postgres.Load()

	_, err := basicPasswordCheck("test@postgres.rest", "123456")
	if err != nil {
		t.Errorf("expected authenticated user, got: %s", err)
	}
}

func Test_getSelectQuery(t *testing.T) {
	config.Load()

	expected := "SELECT * FROM public.prest_users WHERE username=$1 AND password=$2 LIMIT 1"
	query := getSelectQuery()

	if query != expected {
		t.Errorf("expected query: %s, got: %s", expected, query)
	}
}

func Test_encrypt(t *testing.T) {
	config.Load()

	pwd := "123456"
	enc := encrypt(pwd)

	md5Enc := fmt.Sprintf("%x", md5.Sum([]byte(pwd)))
	if enc != md5Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, md5Enc)
	}

	config.PrestConf.AuthEncrypt = "SHA1"

	enc = encrypt(pwd)

	sha1Enc := fmt.Sprintf("%x", sha1.Sum([]byte(pwd)))
	if enc != sha1Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, sha1Enc)
	}
}

func TestAuthDisable(t *testing.T) {
	server := httptest.NewServer(initAuthRoutes())
	defer server.Close()

	t.Log("/auth request POST method, disable auth")
	testutils.DoRequest(t, server.URL+"/auth", nil, "POST", http.StatusNotFound, "AuthDisable")
}

func TestAuthEnable(t *testing.T) {
	config.Load()
	postgres.Load()
	config.PrestConf.AuthEnabled = true

	server := httptest.NewServer(initAuthRoutes())
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
