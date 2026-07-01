package middlewares

import (
	"github.com/rs/cors"
	"github.com/urfave/negroni/v3"

	"github.com/prest/prest/v2/config"
)

// BaseStack returns the default middleware handlers without config-specific layers.
func BaseStack(timeout int) []negroni.Handler {
	return []negroni.Handler{
		negroni.Handler(negroni.NewRecovery()),
		negroni.Handler(negroni.NewLogger()),
		HandlerSet(),
		SetTimeoutToContext(timeout),
	}
}

// New builds the negroni middleware stack from config.
func New(cfg *config.Prest) *negroni.Negroni {
	stack := BaseStack(cfg.HTTPTimeout)

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
		stack = append(
			stack,
			JwtMiddleware(cfg.JWTKey, cfg.JWTJWKS, cfg.JWTAlgo, cfg.JWTWhiteList))
	}
	if cfg.Cache.Enabled {
		stack = append(stack, CacheMiddleware(&cfg.Cache, cfg.JWTWhiteList))
	}
	if cfg.ExposeConf.Enabled {
		stack = append(stack, ExposureMiddleware(cfg.ExposeConf))
	}

	return negroni.New(stack...)
}
