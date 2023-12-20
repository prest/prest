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
func EndpointRulesWithConfig(cfg *config.Prest, uri string) (bool, int) {
	enabled := false
	time := cfg.Cache.Time

	if cfg.Cache.Enabled && len(cfg.Cache.Endpoints) == 0 {
		enabled = true
	}
	for _, endpoint := range cfg.Cache.Endpoints {
		if endpoint.Endpoint == uri {
			enabled = true
			return enabled, endpoint.Time
		}
	}
	return enabled, time
}
