package middlewares

import (
	"github.com/rs/cors"
	"github.com/urfave/negroni/v3"

	"github.com/prest/prest/v2/config"
)

// BaseStack returns the default middleware handlers without config-specific layers.
func BaseStack(timeout int) []negroni.Handler {
	return baseStack(timeout, "")
}

// baseStack returns the default middleware handlers. A non-empty
// requestIDHeader adds RequestIDMiddleware ahead of the access log, which
// then ends each line with the request ID.
func baseStack(timeout int, requestIDHeader string) []negroni.Handler {
	stack := []negroni.Handler{negroni.NewRecovery()}
	logger := negroni.NewLogger()
	if requestIDHeader != "" {
		stack = append(stack, RequestIDMiddleware(requestIDHeader))
		logger.SetFormat(requestIDLogFormat(requestIDHeader))
	}
	return append(stack,
		logger,
		HandlerSet(),
		SetTimeoutToContext(timeout),
	)
}

// New builds the negroni middleware stack from config.
func New(cfg *config.Prest) *negroni.Negroni {
	stack := baseStack(cfg.HTTPTimeout, cfg.HTTPRequestIDHeader)

	// Ahead of CORS so a request for a host pREST does not serve is rejected
	// before any other layer answers it, preflight requests included.
	if cfg.HTTPAllowedHosts != nil {
		stack = append(stack, AllowedHostsMiddleware(cfg.HTTPAllowedHosts))
	}
	if cfg.CORSAllowOrigin != nil {
		stack = append(
			stack,
			cors.New(cors.Options{
				AllowedOrigins:   cfg.CORSAllowOrigin,
				AllowedMethods:   cfg.CORSAllowMethods,
				AllowedHeaders:   cfg.CORSAllowHeaders,
				ExposedHeaders:   exposedHeaders(cfg),
				AllowCredentials: cfg.CORSAllowCredentials,
			}))
	}
	if !cfg.Debug && cfg.EnableDefaultJWT {
		jwtMiddleware, err := JwtMiddleware(
			cfg.JWTKey, cfg.JWTJWKS, cfg.JWTAlgo, cfg.JWTWhiteList)
		if err != nil {
			stack = append(stack, invalidJWTConfigMiddleware(err))
		} else {
			stack = append(stack, jwtMiddleware)
		}
	}
	if cfg.Cache.Enabled {
		stack = append(stack, CacheMiddleware(&cfg.Cache, cfg.JWTWhiteList))
	}
	if cfg.ExposeConf.Enabled {
		stack = append(stack, ExposureMiddleware(cfg.ExposeConf))
	}

	return negroni.New(stack...)
}

// exposedHeaders lists the response headers browser scripts may read on a
// cross-origin response: the request ID header, when one is configured.
func exposedHeaders(cfg *config.Prest) []string {
	if cfg.HTTPRequestIDHeader == "" {
		return nil
	}
	return []string{cfg.HTTPRequestIDHeader}
}
