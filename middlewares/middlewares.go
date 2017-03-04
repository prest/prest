package middlewares

import (
	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/middlewares"
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
		negroni.Handler(middlewares.AccessControl()),
		negroni.Handler(middlewares.HandlerSet()),
	}
)

func initApp() {
	config.InitConf()
	if len(MiddlewareStack) == 0 {
		MiddlewareStack = append(MiddlewareStack, BaseStack...)
	}
	if config.PrestConf.JWTKey != "" {
		MiddlewareStack = append(MiddlewareStack, negroni.Handler(middlewares.JwtMiddleware(config.PrestConf.JWTKey)))
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
