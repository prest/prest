package controllers

import (
	"crypto/md5"
	"crypto/sha1"
	"fmt"
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

	expected := "SELECT * FROM test_users WHERE email=$1 AND password=$2 LIMIT 1"
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
