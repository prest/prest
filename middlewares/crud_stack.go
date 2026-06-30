package middlewares

import (
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/plugins"

	"github.com/urfave/negroni/v3"
)

// CRUDStack bundles middleware applied to CRUD routes.
type CRUDStack struct {
	handlers []negroni.Handler
}

// NewCRUDStack builds the middleware chain for protected table routes.
func NewCRUDStack(cfg *config.Prest) *CRUDStack {
	perms := cfg.Adapter
	return &CRUDStack{
		handlers: []negroni.Handler{
			AuthMiddleware(cfg.JWTAlgo),
			AccessControl(perms),
			ExposureMiddleware(),
			CacheMiddleware(&cfg.Cache),
			plugins.MiddlewarePlugin(),
		},
	}
}

// NewCRUDStackWithPerms builds the CRUD middleware chain with an explicit permissions checker.
func NewCRUDStackWithPerms(cfg *config.Prest, perms adapters.PermissionsChecker) *CRUDStack {
	return &CRUDStack{
		handlers: []negroni.Handler{
			AuthMiddleware(cfg.JWTAlgo),
			AccessControl(perms),
			ExposureMiddleware(),
			CacheMiddleware(&cfg.Cache),
			plugins.MiddlewarePlugin(),
		},
	}
}

// Handlers returns the negroni handlers for this stack.
func (s *CRUDStack) Handlers() []negroni.Handler {
	return s.handlers
}
