package controllers

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
)

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

	expected := "SELECT * FROM prest_users WHERE username=$1 AND password=$2 LIMIT 1"
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
	server := httptest.NewServer(Routes())
	defer server.Close()

	t.Log("/auth request POST method, disable auth")
	doRequest(t, server.URL+"/auth", nil, "POST", http.StatusNotFound, "AuthDisable")
}

func TestAuthEnable(t *testing.T) {
	os.Setenv("PREST_AUTH_ENABLED", "true")
	config.Load()
	postgres.Load()

	server := httptest.NewServer(Routes())
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"/auth request GET method", "/auth", "GET", http.StatusNotFound},
		{"/auth request POST method", "/auth", "POST", http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		doRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "AuthEnable")
	}
}
