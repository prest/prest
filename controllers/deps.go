package controllers

import (
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/cache"
	"github.com/prest/prest/v2/config"
)

// ResponseCacher stores HTTP response payloads for cacheable requests.
type ResponseCacher interface {
	BuntSet(key, value string)
}

// AuthConfig holds authentication settings for AuthHandler.
type AuthConfig struct {
	Enabled  bool
	AuthType string
	JWTKey   string
	Schema   string
	Table    string
	Username string
	Password string
	Encrypt  string
}

// Deps bundles dependencies for HTTP handlers.
type Deps struct {
	Catalog    adapters.CatalogQuerier
	Builder    adapters.RequestQueryBuilder
	Executor   adapters.QueryExecutor
	SQL        adapters.SQLBuilder
	Perms      adapters.PermissionsChecker
	Scripts    adapters.ScriptRunner
	DB         adapters.DatabaseRegistry
	Pinger     adapters.DatabasePinger
	Readiness  adapters.ReadinessChecker
	Cache      ResponseCacher
	SingleDB   bool
	PGDatabase string
	Auth       AuthConfig
}

// NewDepsFromConfig builds handler dependencies from application config.
func NewDepsFromConfig(p *config.Prest) Deps {
	var cacher ResponseCacher
	if p.Cache.Enabled {
		cacher = &p.Cache
	}
	return Deps{
		Catalog:    p.Adapter,
		Builder:    p.Adapter,
		Executor:   p.Adapter,
		SQL:        p.Adapter,
		Perms:      p.Adapter,
		Scripts:    p.Adapter,
		DB:         p.Adapter,
		Pinger:     p.Adapter,
		Readiness:  p.Adapter,
		Cache:      cacher,
		SingleDB:   p.SingleDB,
		PGDatabase: p.PGDatabase,
		Auth: AuthConfig{
			Enabled:  p.AuthEnabled,
			AuthType: p.AuthType,
			JWTKey:   p.JWTKey,
			Schema:   p.AuthSchema,
			Table:    p.AuthTable,
			Username: p.AuthUsername,
			Password: p.AuthPassword,
			Encrypt:  p.AuthEncrypt,
		},
	}
}

// Handlers groups all HTTP handlers for route registration.
type Handlers struct {
	Auth    *AuthHandler
	Catalog *CatalogHandler
	MCP     *MCPHandler
	Table   *TableHandler
	CRUD    *CRUDHandler
	Script  *ScriptHandler
	Health  *HealthHandler
	Ready   *HealthHandler
}

// NewHandlers constructs handlers from dependencies.
func NewHandlers(deps Deps) *Handlers {
	checks := DefaultCheckList(deps.Pinger)
	return &Handlers{
		Auth:    NewAuthHandler(deps.Executor, deps.Auth),
		Catalog: NewCatalogHandler(deps),
		MCP:     NewMCPHandler(deps),
		Table:   NewTableHandler(deps.Executor, deps.DB, deps.SingleDB),
		CRUD:    NewCRUDHandler(deps),
		Script:  NewScriptHandler(deps),
		Health:  NewHealthHandler(checks),
		Ready:   NewHealthHandler(DefaultReadyCheckList(deps.Readiness)),
	}
}

// NewHandlersFromConfig builds handlers from application config.
func NewHandlersFromConfig(p *config.Prest) *Handlers {
	return NewHandlers(NewDepsFromConfig(p))
}

// Ensure cache.Config satisfies ResponseCacher.
var _ ResponseCacher = (*cache.Config)(nil)
