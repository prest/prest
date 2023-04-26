package middlewares

import (
	"github.com/prest/prest/config"
	"github.com/rs/cors"
	"github.com/urfave/negroni/v3"
)

var (
	app *negroni.Negroni

	// MiddlewareStack on pREST
	MiddlewareStack []negroni.Handler

	// BaseStack Middlewares
	BaseStack = []negroni.Handler{
		negroni.Handler(negroni.NewRecovery()),
		negroni.Handler(negroni.NewLogger()),
		HandlerSet(),
		SetTimeoutToContext(),
	}
)

func initApp() {
	if len(MiddlewareStack) == 0 {
		MiddlewareStack = append(MiddlewareStack, BaseStack...)
		if config.PrestConf.CORSAllowOrigin != nil {
			MiddlewareStack = append(
				MiddlewareStack,
				cors.New(cors.Options{
					AllowedOrigins:   config.PrestConf.CORSAllowOrigin,
					AllowedMethods:   config.PrestConf.CORSAllowMethods,
					AllowedHeaders:   config.PrestConf.CORSAllowHeaders,
					AllowCredentials: config.PrestConf.CORSAllowCredentials,
				}))
		}
		if !config.PrestConf.Debug && config.PrestConf.EnableDefaultJWT {
			MiddlewareStack = append(
				MiddlewareStack,
				JwtMiddleware(config.PrestConf.JWTKey, config.PrestConf.JWTAlgo))
		}
		if config.PrestConf.Cache.Enabled {
			MiddlewareStack = append(MiddlewareStack, CacheMiddleware())
		}
		if config.PrestConf.ExposeConf.Enabled {
			MiddlewareStack = append(MiddlewareStack, ExposureMiddleware())
		}
	}
	app = negroni.New(MiddlewareStack...)
}

// GetApp get negroni
func GetApp() *negroni.Negroni {
	if app == nil {
		initApp()
	}
	return app
}
