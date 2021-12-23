package middlewares

import (
	"fmt"
	"net/http"

	"github.com/prest/prest/cache"
	"github.com/prest/prest/config"
	"github.com/urfave/negroni"
)

// CacheMiddleware simple caching to avoid equal queries to the database
func CacheMiddleware() negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(r.URL.String())
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}
		if config.PrestConf.Cache && r.Method == "GET" && !match {
			if cache.BuntGet(r.URL.String(), w) {
				return
			}
		}
		next(w, r)
	})
}
