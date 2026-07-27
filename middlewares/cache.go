package middlewares

import (
	"fmt"
	"net/http"

	"github.com/urfave/negroni/v3"

	"github.com/prest/prest/v2/cache"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"
)

// CacheKey builds the per-identity cache key for a request. The cache sits
// downstream of AccessControl but upstream of the handler's own per-user
// FieldsPermissions (column-level) filtering: on a cache hit the handler never
// runs, so a key scoped only by URL would let one user's cached response
// (built under their own column permissions) leak to a different user with
// different field permissions on the same table.
//
// The identity is appended after a synthetic "?", not merged into the path:
// cache.Config.BuntSet recovers the endpoint path for its own cache-rule
// lookup via strings.Split(key, "?")[0], so whatever follows the first "?"
// — real query string or not — must stay irrelevant to that split.
func CacheKey(r *http.Request) string {
	userName := ""
	if userInfo := r.Context().Value(pctx.UserInfoKey); userInfo != nil {
		if user, ok := userInfo.(auth.User); ok {
			userName = user.Username
		}
	}
	return r.URL.String() + "?__prest_user=" + userName
}

// CacheMiddleware simple caching to avoid equal queries to the database
// todo: receive config.PrestConf.Cache to pass to cache.EndpointRules
// this will help removing global config calls
func CacheMiddleware(cfg *cache.Config, whitelist []string) negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(r.URL.String(), whitelist)
		if err != nil {
			http.Error(w, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusInternalServerError)
			return
		}
		// team will not be used when downloading information, second result ignored
		cacheRule, _ := cfg.EndpointRules(r.URL.Path)
		if cfg.Enabled && r.Method == "GET" && !match && cacheRule {
			if cfg.BuntGet(CacheKey(r), w) {
				return
			}
		}
		next(w, r)
	})
}
