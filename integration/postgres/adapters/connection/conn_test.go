package connection_test

import (
	"testing"

	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/integration/helpers"
)

func TestGet(t *testing.T) {
	cfg := helpers.LoadTestConfig(t)
	t.Log("Open connection")
	db, err := postgres.DB(cfg.Adapter)
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
	cfg := helpers.LoadTestConfig(t)
	t.Log("Open connection")
	db, err := postgres.DB(cfg.Adapter)
	if err != nil {
		t.Fatalf("expected no error opening db, but got: %v", err)
	}
	if db == nil {
		t.Fatalf("expected db connection, but no was!")
	}

	t.Log("Ping Pong")
	err = db.Ping()
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}
}
