package cache

import (
	"os"
	"testing"

	"github.com/prest/prest/config"
)

func init() {
	os.Setenv("PREST_CONF", "./testdata/prest.toml")
	os.Setenv("PREST_CACHE_ENABLED", "true")
	os.Setenv("PREST_PG_CACHE", "true")
	config.Load()
}
func TestEndpointRulesEnable(t *testing.T) {
	config.PrestConf.Cache.Endpoints = append(config.PrestConf.Cache.Endpoints, config.CacheEndpoint{
		Time:     5,
		Endpoint: "/prest/public/test",
		Enabled:  true,
	})
	cacheEnable, cacheTime := EndpointRules("/prest/public/test")
	if !cacheEnable {
		t.Errorf("expected cache endpoint rule true, but got %t", cacheEnable)
	}
	if cacheTime != 5 {
		t.Errorf("expected cache endpoint time 5, but got %d", cacheTime)
	}
}

func TestEndpointRulesNotExist(t *testing.T) {
	cacheEnable, _ := EndpointRules("/prest/public/test-notexist")
	if cacheEnable {
		t.Errorf("expected cache endpoint rule false, but got %t", cacheEnable)
	}
}

func TestEndpointRulesDisable(t *testing.T) {
	config.PrestConf.Cache.Endpoints = append(config.PrestConf.Cache.Endpoints, config.CacheEndpoint{
		Endpoint: "/prest/public/test-disable",
		Enabled:  false,
	})
	cacheEnable, cacheTime := EndpointRules("/prest/public/test-diable")
	if cacheEnable {
		t.Errorf("expected cache endpoint rule false, but got %t", cacheEnable)
	}
	if cacheTime == 10 {
		t.Errorf("expected cache endpoint time is nil, but got %d", cacheTime)
	}
}
