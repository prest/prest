package middlewares

import (
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"

	"github.com/urfave/negroni/v3"
)

// CatalogStack bundles middleware applied to catalog/metadata routes.
type CatalogStack struct {
	handlers []negroni.Handler
}

// NewCatalogStack builds the chain for catalog/metadata routes: AuthMiddleware
// (enforces auth.enabled) then AccessControl, which applies the table ACL to
// /show/{database}/{schema}/{table} and passes the non-table listings through.
func NewCatalogStack(cfg *config.Prest, perms adapters.PermissionsChecker) *CatalogStack {
	return &CatalogStack{
		handlers: []negroni.Handler{
			AuthMiddleware(AuthSettings{
				Enabled:      cfg.AuthEnabled,
				JWTKey:       cfg.JWTKey,
				JWTWhiteList: cfg.JWTWhiteList,
			}),
			AccessControl(perms),
		},
	}
}

// Handlers returns the negroni handlers for this stack.
func (s *CatalogStack) Handlers() []negroni.Handler {
	return s.handlers
}
