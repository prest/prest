package connection

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	config "github.com/prest/config"
)

func init() {
	config.Load()
}

func TestGet(t *testing.T) {
	t.Log("Open connection")
	db, err := Get()
	if err != nil {
		t.Fatalf("Expected err equal to nil but got %q", err.Error())
	}

	t.Log("Ping Pong")
	err = db.Ping()
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}
}

func TestMustGet(t *testing.T) {
	t.Log("Open connection")
	db := MustGet()
	if db == nil {
		t.Fatalf("expected db connection, but no was!")
	}

	t.Log("Ping Pong")
	err := db.Ping()
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
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
