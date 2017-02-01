package connection

import (
	"testing"
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
