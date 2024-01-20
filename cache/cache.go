package cache

import (
	"net/http"

	"github.com/prest/prest/cache/bunt"
	"github.com/prest/prest/config"
)

type Cacher interface {
	Get(key string, w http.ResponseWriter) (cacheExist bool)
	Set(key, value string)
	EndpointRules(uri string) (bool, int)
}

// New creates a new cacher instance with the given configuration and logger.
// It initializes the adapter based on the provided configuration.
// Returns a pointer to the newly created Config instance and an error if any.
func New(cfg *config.CacheConf) Cacher {
	return bunt.New(cfg)
}
