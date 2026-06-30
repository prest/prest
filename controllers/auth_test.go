package controllers

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"testing"
)

func testAuthHandler() *AuthHandler {
	return NewAuthHandler(nil, AuthConfig{
		Schema:   "public",
		Table:    "prest_users",
		Username: "username",
		Password: "password",
		Encrypt:  "MD5",
	})
}

func Test_getSelectQuery(t *testing.T) {
	expected := "SELECT * FROM public.prest_users WHERE username=$1 AND password=$2 LIMIT 1"
	query := testAuthHandler().selectQuery()

	if query != expected {
		t.Errorf("expected query: %s, got: %s", expected, query)
	}
}

func Test_encrypt(t *testing.T) {
	h := testAuthHandler()
	pwd := "123456"
	enc := h.encrypt(pwd)

	md5Enc := fmt.Sprintf("%x", md5.Sum([]byte(pwd)))
	if enc != md5Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, md5Enc)
	}

	h.cfg.Encrypt = "SHA1"
	enc = h.encrypt(pwd)

	sha1Enc := fmt.Sprintf("%x", sha1.Sum([]byte(pwd)))
	if enc != sha1Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, sha1Enc)
	}

	_ = sha1Enc
}
