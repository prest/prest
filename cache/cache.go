package cache

import (
	"github.com/prest/prest/config"
)

// EndpointRules checks if there is a custom caching rule for the endpoint
func EndpointRules(uri string) (cacheEnable bool, time int) {
	cacheEnable = false
	if config.PrestConf.Cache.Enabled && len(config.PrestConf.Cache.Endpoints) == 0 {
		cacheEnable = true
	}

	time = config.PrestConf.Cache.Time
	for _, endpoint := range config.PrestConf.Cache.Endpoints {
		if endpoint.Endpoint == uri {
			cacheEnable = true
			time = endpoint.Time
			return
		}
	}
	return
}
