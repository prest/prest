package cache

import (
	"github.com/prest/prest/config"
)

// EndpointRules checks if there is a custom caching rule for the endpoint
func EndpointRules(uri string) (cacheEnable bool, time int) {
	cacheEnable = false
	if config.PrestConf.Cache && len(config.PrestConf.CacheEndpoints) == 0 {
		cacheEnable = true
	}
	time = config.PrestConf.CacheTime
	for _, endpoint := range config.PrestConf.CacheEndpoints {
		if endpoint.Endpoint == uri {
			cacheEnable = true
			time = endpoint.CacheTime
			return
		}
	}
	return
}
