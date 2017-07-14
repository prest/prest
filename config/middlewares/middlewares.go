package middlewares

import (
	"fmt"

	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/middlewares"
	"github.com/rs/cors"
	"github.com/urfave/negroni"
)

var (
	app *negroni.Negroni

	// MiddlewareStack on pREST
	MiddlewareStack []negroni.Handler

	// BaseStack Middlewares
	BaseStack = []negroni.Handler{
		negroni.Handler(negroni.NewRecovery()),
		negroni.Handler(negroni.NewLogger()),
		negroni.Handler(middlewares.HandlerSet()),
	}
)

func initApp() {
	if len(MiddlewareStack) == 0 {
		MiddlewareStack = append(MiddlewareStack, BaseStack...)
	}
	if !config.PrestConf.Debug {
		MiddlewareStack = append(MiddlewareStack, negroni.Handler(middlewares.JwtMiddleware(config.PrestConf.JWTKey)))
	}
	if config.PrestConf.CORSAllowOrigin != nil {
		fmt.Println("Allow origin ", config.PrestConf.CORSAllowOrigin)
		c := cors.New(cors.Options{
			AllowedOrigins: config.PrestConf.CORSAllowOrigin,
		})
		MiddlewareStack = append(MiddlewareStack, negroni.Handler(c))
	}
	app = negroni.New(MiddlewareStack...)
}

// GetApp get negroni
func GetApp() *negroni.Negroni {
	// init application every time
	initApp()
	return app
}
