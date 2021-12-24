package cache

import (
	"log"

	"github.com/prest/prest/config"
)

// CacheEndpointRules ...
func CacheEndpointRules(uri string) (cacheEnable bool, time int) {
	cacheEnable = false
	if config.PrestConf.Cache && len(config.PrestConf.CacheEndpoints) == 0 {
		cacheEnable = true
	}
	time = config.PrestConf.CacheTime
	log.Println("cache len:", cacheEnable, config.PrestConf.CacheEndpoints, len(config.PrestConf.CacheEndpoints))
	for _, endpoint := range config.PrestConf.CacheEndpoints {
		if endpoint.Endpoint == uri {
			cacheEnable = true
			time = endpoint.CacheTime
			return
		}
	}
	return
}
