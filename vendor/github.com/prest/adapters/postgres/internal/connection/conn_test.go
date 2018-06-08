package connection

import (
	"testing"

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
