package bunt

import (
	"testing"

	cf "github.com/prest/prest/config"
	"github.com/stretchr/testify/require"
)

var (
	cfg = &config{
		prestcfg: cf.CacheConf{
			Enabled:     true,
			Time:        10,
			Endpoints:   []cf.Endpoint{},
			StoragePath: "./",
		},
	}
)

func TestEndpointRulesEnable(t *testing.T) {
	cfg.prestcfg.Endpoints = append(cfg.prestcfg.Endpoints, cf.Endpoint{
		Time:     5,
		Endpoint: "/prest/public/test",
		Enabled:  true,
	})
	cacheEnable, cacheTime := cfg.EndpointRules("/prest/public/test")
	require.True(t, cacheEnable)
	require.Equal(t, 5, cacheTime)
	cfg.ClearEndpoints()
}

func TestEndpointRulesNotExist(t *testing.T) {
	cfg.prestcfg.Endpoints = append(cfg.prestcfg.Endpoints, cf.Endpoint{
		Time:     5,
		Endpoint: "/prest/public/something",
		Enabled:  true,
	})
	cacheEnable, _ := cfg.EndpointRules("/prest/public/test-notexist")
	require.False(t, cacheEnable)
	cfg.ClearEndpoints()
}

func TestEndpointRulesNotExistWithoutEndpoints(t *testing.T) {
	cacheEnable, cacheTime := cfg.EndpointRules("/prest/public/test-notexist")
	require.True(t, cacheEnable)
	require.Equal(t, 10, cacheTime)
	cfg.ClearEndpoints()
}

func TestEndpointRulesDisable(t *testing.T) {
	cfg.prestcfg.Endpoints = append(cfg.prestcfg.Endpoints, cf.Endpoint{
		Endpoint: "/prest/public/test-disable",
		Enabled:  false,
	})
	cacheEnable, cacheTime := cfg.EndpointRules("/prest/public/test-diable")
	require.False(t, cacheEnable)
	require.Equal(t, 10, cacheTime)
	cfg.ClearEndpoints()
}
