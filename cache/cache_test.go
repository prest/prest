package cache

import (
	"os"
	"testing"

	"github.com/prest/prest/config"
)

func TestEndpointRulesEnable(t *testing.T) {
	os.Setenv("PREST_CONF", "./testdata/prest.toml")
	os.Setenv("PREST_CACHE", "true")
	config.Load()
	cacheEnable, cacheTime := EndpointRules("/prest/public/test")
	if !cacheEnable {
		t.Errorf("expected cache endpoint rule true, but got %t", cacheEnable)
	}
	if cacheTime != 5 {
		t.Errorf("expected cache endpoint time 5, but got %d", cacheTime)
	}
}

func TestEndpointRulesDisable(t *testing.T) {
	os.Setenv("PREST_CACHE", "true")
	config.Load()
	cacheEnable, _ := EndpointRules("/prest/public/test-disable")
	if cacheEnable {
		t.Errorf("expected cache endpoint rule false, but got %t", cacheEnable)
	}
}
