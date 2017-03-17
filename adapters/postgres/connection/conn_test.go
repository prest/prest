package connection

import (
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/nuveo/prest/adapters/postgres"
)

func TestMustGet(t *testing.T) {
	t.Log("Open connection")
	db := MustGet()
	if db == nil {
		t.Errorf("expected db connection, but no was!")
	}

	t.Log("Ping Pong")
	err := db.Ping()
	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}
}

func TestSetNativeDB(t *testing.T) {
	t.Log("Open connection")
	db := MustGet()
	if db == nil {
		t.Errorf("expected db connection, but no was!")
	}
	mockedDB, _, err := sqlmock.New()
	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}
	SetNativeDB(mockedDB)
	if db.DB != mockedDB {
		t.Errorf("expected same memory address, but no was! %v %v", db.DB, mockedDB)
	}
}

func TestTimeout(t *testing.T) {
	_, err := postgres.Query("SET statement_timeout TO 10;")
	if err != nil {
		t.Errorf("Error setting statement_timeout: %s", err)
	}

	_, err = postgres.Query("SELECT pg_sleep(1000);")
	if err == nil {
		t.Errorf("Error should not be nil")
	}

	if !strings.Contains(err.Error(), "statement timeout") {
		t.Errorf("Returned different error: %s", err)
	}

	_, err = postgres.Query("SET statement_timeout TO 0;")
	if err != nil {
		t.Errorf("Error disabling statement_timeout")
	}
}
