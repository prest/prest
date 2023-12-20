package middlewares

import (
	"fmt"
	"net/http"

	"github.com/prest/prest/cache"
	"github.com/prest/prest/config"
	"github.com/urfave/negroni/v3"
)

// CacheMiddleware simple caching to avoid equal queries to the database
// todo: receive config.PrestConf.Cache to pass to cache.EndpointRules
// this will help removing global config calls
func CacheMiddleware() negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(r.URL.String())
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}
		// team will not be used when downloading information, second result ignored
		cacheRule, _ := cache.EndpointRules(r.URL.Path)
		if config.PrestConf.Cache.Enabled && r.Method == "GET" && !match && cacheRule {
			if cache.BuntGet(r.URL.String(), w) {
				return
			}
		}
		next(w, r)
	})
}
