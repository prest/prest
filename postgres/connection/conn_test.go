package connection

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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
