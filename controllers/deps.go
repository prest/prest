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
	Catalog       adapters.CatalogQuerier
	Builder       adapters.RequestQueryBuilder
	Executor      adapters.QueryExecutor
	SQL           adapters.SQLBuilder
	Perms         adapters.PermissionsChecker
	Scripts       adapters.ScriptRunner
	QueryRegistry adapters.QueryRegistry
	ScriptPerms   adapters.ScriptPermissionsChecker
	DB            adapters.DatabaseRegistry
	// TxManager opens transactions for the /_transactions endpoints. Kept as its
	// own field rather than reaching through DB, which is the multi-database
	// registry and does not carry transaction methods.
	TxManager adapters.TransactionManager
	// TxExec is the adapter's transaction-aware write path, nil if the adapter
	// does not implement TxExecutor.
	TxExec TxExecutor
	// TxHandler resolves X-Prest-Transaction to an open transaction. It is
	// built here rather than in NewHandlers so CRUD and the /_transactions
	// routes share one manager, and it is nil in hand-built test Deps, which
	// correctly makes a transaction header a 501 instead of a silent no-op.
	TxHandler       *TransactionHandler
	Pinger          adapters.DatabasePinger
	Readiness       adapters.ReadinessChecker
	Cache           ResponseCacher
	AdapterRegistry adapters.Registry // Multi-database adapter registry
	SingleDB        bool
	PGDatabase      string
	Auth            AuthConfig
	Expose          config.ExposeConf
}

// NewDepsFromConfig builds handler dependencies from application config.
func NewDepsFromConfig(p *config.Prest) Deps {
	var cacher ResponseCacher
	if p.Cache.Enabled {
		cacher = &p.Cache
	}
	var queryRegistry adapters.QueryRegistry
	var scriptPerms adapters.ScriptPermissionsChecker
	var txExec TxExecutor
	if reg, ok := p.Adapter.(adapters.QueryRegistry); ok {
		queryRegistry = reg
	}
	if perms, ok := p.Adapter.(adapters.ScriptPermissionsChecker); ok {
		scriptPerms = perms
	}
	if tx, ok := p.Adapter.(TxExecutor); ok {
		txExec = tx
	}
	return Deps{
		Catalog:       p.Adapter,
		Builder:       p.Adapter,
		Executor:      p.Adapter,
		SQL:           p.Adapter,
		Perms:         p.Adapter,
		Scripts:       p.Adapter,
		QueryRegistry: queryRegistry,
		ScriptPerms:   scriptPerms,
		DB:            p.Adapter,
		TxManager:     p.Adapter,
		TxExec:        txExec,
		TxHandler:     NewTransactionHandler(p.Adapter),
		Pinger:        p.Adapter,
		Readiness:     p.Adapter,
		Cache:         cacher,
		SingleDB:      p.SingleDB,
		PGDatabase:    p.PGDatabase,
		Expose:        p.ExposeConf,
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
	Auth          *AuthHandler
	Catalog       *CatalogHandler
	MCP           *MCPHandler
	Table         *TableHandler
	CRUD          *CRUDHandler
	Script        *ScriptHandler
	QueryRegistry *QueryRegistryHandler
	Health        *HealthHandler
	Ready         *HealthHandler
	Transaction   *TransactionHandler
}

// NewHandlers constructs handlers from dependencies.
func NewHandlers(deps Deps, cfg *config.Prest) *Handlers {
	checks := DefaultCheckList(deps.Pinger)
	// Reuse the handler built in NewDepsFromConfig so CRUD and the
	// /_transactions routes resolve the same open transactions. A hand-built
	// Deps in a test has none, so build one from the manager.
	txHandler := deps.TxHandler
	if txHandler == nil && deps.TxManager != nil {
		txHandler = NewTransactionHandler(deps.TxManager)
	}
	h := &Handlers{
		Auth:        NewAuthHandler(deps.Executor, deps.Auth),
		Catalog:     NewCatalogHandler(deps),
		MCP:         NewMCPHandler(deps),
		Table:       NewTableHandler(deps.Executor, deps.DB, deps.SingleDB),
		CRUD:        NewCRUDHandler(deps),
		Transaction: txHandler,
		Script:      NewScriptHandler(deps),
		Health:      NewHealthHandler(checks),
		Ready:       NewHealthHandler(DefaultReadyCheckList(deps.Readiness)),
	}
	if cfg != nil && deps.QueryRegistry != nil && cfg.QueriesConf.RegisterEnabled && cfg.QueriesConf.Storage == config.QueriesStorageDatabase {
		h.QueryRegistry = NewQueryRegistryHandler(deps, cfg.QueriesConf)
	}
	return h
}

// NewHandlersFromConfig builds handlers from application config.
func NewHandlersFromConfig(p *config.Prest) *Handlers {
	return NewHandlers(NewDepsFromConfig(p), p)
}

// Ensure cache.Config satisfies ResponseCacher.
var _ ResponseCacher = (*cache.Config)(nil)
