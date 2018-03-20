package middlewares

import (
	"github.com/prest/config"
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
		HandlerSet(),
	}
)

func initApp() {
	if len(MiddlewareStack) == 0 {
		MiddlewareStack = append(MiddlewareStack, BaseStack...)
	}
	if !config.PrestConf.Debug && config.PrestConf.EnableDefaultJWT {
		MiddlewareStack = append(MiddlewareStack, JwtMiddleware(config.PrestConf.JWTKey, config.PrestConf.JWTAlgo))
	}
	if config.PrestConf.CORSAllowOrigin != nil {
		MiddlewareStack = append(MiddlewareStack, Cors(config.PrestConf.CORSAllowOrigin, config.PrestConf.CORSAllowHeaders))
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
