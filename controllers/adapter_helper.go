package controllers

import (
	"net/http"

	"github.com/prest/prest/v2/adapters"
	pctx "github.com/prest/prest/v2/context"
)

// GetAdapterForRequest retrieves the appropriate adapter for a request.
// For multi-database setups, this returns the adapter selected by the
// AdapterSelectorMiddleware from the request context.
// For single-database setups, this returns the default adapter.
func GetAdapterForRequest(r *http.Request, defaultAdapter adapters.Adapter) adapters.Adapter {
	// Check if adapter was attached to context by AdapterSelectorMiddleware (multi-DB mode)
	if adapter, ok := r.Context().Value(pctx.AdapterKey).(adapters.Adapter); ok {
		return adapter
	}
	// Fall back to default adapter (single-DB mode)
	return defaultAdapter
}

// GetAdapterFromRegistry retrieves an adapter from the registry for a specific database.
// Returns an error if the database is not registered.
func GetAdapterFromRegistry(registry adapters.Registry, database string) (adapters.Adapter, error) {
	if registry == nil {
		return nil, adapters.ErrAdapterNotFound
	}
	return registry.Get(database)
}
