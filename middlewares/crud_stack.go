package middlewares

import (
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/plugins"
	"github.com/prest/prest/v2/transactions"

	"github.com/urfave/negroni/v3"
)

// CRUDStack bundles middleware applied to CRUD routes.
type CRUDStack struct {
	handlers []negroni.Handler
}

// NewCRUDStack builds the middleware chain for protected table routes.
func NewCRUDStack(cfg *config.Prest, plg *plugins.Plugins, txMgr *transactions.Manager) *CRUDStack {
	perms := cfg.Adapter
	handlers := []negroni.Handler{
		AuthMiddleware(AuthSettings{
			Enabled:      cfg.AuthEnabled,
			JWTKey:       cfg.JWTKey,
			JWTWhiteList: cfg.JWTWhiteList,
		}),
	}
	if txMgr != nil {
		handlers = append(handlers, TransactionMiddleware(txMgr))
	}
	handlers = append(handlers,
		AccessControl(perms),
		ExposureMiddleware(cfg.ExposeConf),
		CacheMiddleware(&cfg.Cache, cfg.JWTWhiteList),
		plg.Middleware(),
	)
	return &CRUDStack{handlers: handlers}
}

// NewCRUDStackWithPerms builds the CRUD middleware chain with an explicit permissions checker.
func NewCRUDStackWithPerms(cfg *config.Prest, plg *plugins.Plugins, perms adapters.PermissionsChecker, txMgr *transactions.Manager) *CRUDStack {
	handlers := []negroni.Handler{
		AuthMiddleware(AuthSettings{
			Enabled:      cfg.AuthEnabled,
			JWTKey:       cfg.JWTKey,
			JWTWhiteList: cfg.JWTWhiteList,
		}),
	}
	if txMgr != nil {
		handlers = append(handlers, TransactionMiddleware(txMgr))
	}
	handlers = append(handlers,
		AccessControl(perms),
		ExposureMiddleware(cfg.ExposeConf),
		CacheMiddleware(&cfg.Cache, cfg.JWTWhiteList),
		plg.Middleware(),
	)
	return &CRUDStack{handlers: handlers}
}

// Handlers returns the negroni handlers for this stack.
func (s *CRUDStack) Handlers() []negroni.Handler {
	return s.handlers
}
