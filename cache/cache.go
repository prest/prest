package cache

import (
	"github.com/prest/prest/config"
)

// EndpointRules checks if there is a custom caching rule for the endpoint
// todo: deprecate
func EndpointRules(uri string) (cacheEnable bool, time int) {
	return EndpointRulesWithConfig(config.PrestConf, uri)
}

// EndpointRulesWithConfig checks if there is a custom caching rule for the endpoint
func EndpointRulesWithConfig(cfg *config.Prest, uri string) (cacheEnable bool, time int) {
	cacheEnable = false
	if cfg.Cache.Enabled && len(cfg.Cache.Endpoints) == 0 {
		cacheEnable = true
	}

	for _, endpoint := range cfg.Cache.Endpoints {
		if endpoint.Endpoint == uri {
			cacheEnable = true
			time = endpoint.Time
			return
		}
	}
	return
}
