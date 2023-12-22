package middlewares

import (
	"fmt"
	"net/http"

	"github.com/prest/prest/config"
	"github.com/urfave/negroni/v3"
)

// CacheMiddleware simple caching to avoid equal queries to the database
// todo: receive config.PrestConf.Cache to pass to cache.EndpointRules
// this will help removing global config calls
func CacheMiddleware(cfg *config.Prest) negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(cfg.JWTWhiteList, r.URL.String())
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}
		// team will not be used when downloading information, second result ignored
		cacheRule, _ := cfg.Cache.EndpointRules(r.URL.Path)
		if cfg.Cache.Enabled && r.Method == "GET" && !match && cacheRule {
			if cfg.Cache.BuntGet(r.URL.String(), w) {
				return
			}
		}
		next(w, r)
	})
}
