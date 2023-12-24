package middlewares

import (
	"net/http"

	"github.com/prest/prest/config"

	"github.com/rs/cors"
	"github.com/urfave/negroni/v3"
)

type OptMiddleware func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc)

var (
	// BaseStack Middlewares
	BaseStack = []negroni.Handler{
		negroni.Handler(negroni.NewRecovery()),
		negroni.Handler(negroni.NewLogger()),
		HandlerSet(),
	}
)

// GetApp gets the default negroni app
func GetApp(cfg *config.Prest, opts ...OptMiddleware) *negroni.Negroni {
	stack := []negroni.Handler{}
	stack = append(stack, BaseStack...)
	stack = append(stack, SetTimeoutToContext(cfg.HTTPTimeout))

	if cfg.CORSAllowOrigin != nil {
		stack = append(
			stack,
			cors.New(cors.Options{
				AllowedOrigins:   cfg.CORSAllowOrigin,
				AllowedMethods:   cfg.CORSAllowMethods,
				AllowedHeaders:   cfg.CORSAllowHeaders,
				AllowCredentials: cfg.CORSAllowCredentials,
			}))
	}
	if !cfg.Debug && cfg.EnableDefaultJWT {
		stack = append(stack, JwtMiddleware(cfg.JWTKey, cfg.JWTWhiteList))
	}
	if cfg.Cache.Enabled {
		stack = append(stack, CacheMiddleware(cfg))
	}
	if cfg.ExposeConf.Enabled {
		stack = append(stack, ExposureMiddleware(cfg))
	}
	for _, opt := range opts {
		stack = append(stack, negroni.HandlerFunc(opt))
	}
	return negroni.New(stack...)
}
