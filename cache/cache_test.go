package cache

import (
	"os"
	"testing"

	"github.com/prest/prest/config"
)

func TestCacheEndpointRulesEnable(t *testing.T) {
	os.Setenv("PREST_CONF", "./testdata/prest.toml")
	os.Setenv("PREST_CACHE", "true")
	config.Load()
	cacheEnable, cacheTime := CacheEndpointRules("/prest/public/test")
	if !cacheEnable {
		t.Errorf("expected cache endpoint rule true, but got %t", cacheEnable)
	}
	if cacheTime != 5 {
		t.Errorf("expected cache endpoint time 5, but got %d", cacheTime)
	}
}

func TestCacheEndpointRulesDisable(t *testing.T) {
	os.Setenv("PREST_CACHE", "true")
	config.Load()
	cacheEnable, _ := CacheEndpointRules("/prest/public/test-disable")
	if cacheEnable {
		t.Errorf("expected cache endpoint rule false, but got %t", cacheEnable)
	}
}
