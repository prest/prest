package middlewares

import (
	"github.com/prest/config"
	"github.com/prest/middlewares"
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
		middlewares.HandlerSet(),
	}
)

func initApp() {
	if len(MiddlewareStack) == 0 {
		MiddlewareStack = append(MiddlewareStack, BaseStack...)
	}
	if !config.PrestConf.Debug {
		MiddlewareStack = append(MiddlewareStack, middlewares.JwtMiddleware(config.PrestConf.JWTKey))
	}
	if config.PrestConf.CORSAllowOrigin != nil {
		MiddlewareStack = append(MiddlewareStack, middlewares.Cors(config.PrestConf.CORSAllowOrigin))
	}
	app = negroni.New(MiddlewareStack...)
}

// GetApp get negroni
func GetApp() *negroni.Negroni {
	// init application every time
	initApp()
	return app
}
