package cache

import (
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
)

func init() {
	config.Load()
	postgres.Load()
}

func TestBuntGetDoesntExist(t *testing.T) {
	t.Setenv("PREST_CACHE", "true")
	config.Load()
	w := httptest.NewRecorder()

	cache := BuntGet("test", w)
	if cache {
		t.Errorf("expected cache non-existent, but got %t", cache)
	}
}
