package cache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func testConfig() *Config {
	return &Config{
		Enabled:     true,
		Time:        10,
		Endpoints:   []Endpoint{},
		StoragePath: "./",
	}
}

func TestEndpointRulesEnable(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Endpoints = append(cfg.Endpoints, Endpoint{
		Time:     5,
		Endpoint: "/prest/public/test",
		Enabled:  true,
	})
	cacheEnable, cacheTime := cfg.EndpointRules("/prest/public/test")
	require.True(t, cacheEnable)
	require.Equal(t, 5, cacheTime)
}

func TestEndpointRulesNotExist(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Endpoints = append(cfg.Endpoints, Endpoint{
		Time:     5,
		Endpoint: "/prest/public/something",
		Enabled:  true,
	})
	cacheEnable, _ := cfg.EndpointRules("/prest/public/test-notexist")
	require.False(t, cacheEnable)
}

func TestEndpointRulesNotExistWithoutEndpoints(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cacheEnable, cacheTime := cfg.EndpointRules("/prest/public/test-notexist")
	require.True(t, cacheEnable)
	require.Equal(t, 10, cacheTime)
}

func TestEndpointRulesDisable(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Endpoints = append(cfg.Endpoints, Endpoint{
		Endpoint: "/prest/public/test-disable",
		Enabled:  false,
	})
	cacheEnable, cacheTime := cfg.EndpointRules("/prest/public/test-diable")
	require.False(t, cacheEnable)
	require.Equal(t, 10, cacheTime)
}
