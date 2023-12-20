package cache

import (
	"testing"

	"github.com/prest/prest/config"

	"github.com/stretchr/testify/require"
)

var (
	cfg = &config.Prest{
		Cache: config.Cache{
			Enabled:     true,
			Time:        10,
			Endpoints:   []config.CacheEndpoint{},
			StoragePath: "./",
		},
	}
)

func TestEndpointRulesEnable(t *testing.T) {
	cfg.Cache.Endpoints = append(cfg.Cache.Endpoints, config.CacheEndpoint{
		Time:     5,
		Endpoint: "/prest/public/test",
		Enabled:  true,
	})
	cacheEnable, cacheTime := EndpointRulesWithConfig(cfg, "/prest/public/test")
	require.True(t, cacheEnable)
	require.Equal(t, 5, cacheTime)
	cfg.Cache.ClearEndpoints()
}

func TestEndpointRulesNotExist(t *testing.T) {
	cfg.Cache.Endpoints = append(cfg.Cache.Endpoints, config.CacheEndpoint{
		Time:     5,
		Endpoint: "/prest/public/something",
		Enabled:  true,
	})
	cacheEnable, _ := EndpointRulesWithConfig(cfg, "/prest/public/test-notexist")
	require.False(t, cacheEnable)
	cfg.Cache.ClearEndpoints()
}

func TestEndpointRulesNotExistWithoutEndpoints(t *testing.T) {
	cacheEnable, cacheTime := EndpointRulesWithConfig(cfg, "/prest/public/test-notexist")
	require.True(t, cacheEnable)
	require.Equal(t, 10, cacheTime)
	cfg.Cache.ClearEndpoints()
}

func TestEndpointRulesDisable(t *testing.T) {
	cfg.Cache.Endpoints = append(cfg.Cache.Endpoints, config.CacheEndpoint{
		Endpoint: "/prest/public/test-disable",
		Enabled:  false,
	})
	cacheEnable, cacheTime := EndpointRulesWithConfig(cfg, "/prest/public/test-diable")
	require.False(t, cacheEnable)
	require.Equal(t, 10, cacheTime)
	cfg.Cache.ClearEndpoints()
}
