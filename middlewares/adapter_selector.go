package middlewares

import (
	"context"
	"net/http"

	"github.com/prest/prest/v2/adapters"
	pctx "github.com/prest/prest/v2/context"
)

// AdapterSelectorMiddleware selects the correct adapter based on the database name
// in the request URL path. For multi-database setups, this routes each request
// to the appropriate adapter (Postgres, TimescaleDB, MySQL, etc.).
type AdapterSelectorMiddleware struct {
	registry adapters.Registry
	next     http.Handler
}

// NewAdapterSelectorMiddleware creates middleware that routes requests by database.
// registry: adapter registry mapping database aliases to adapter instances
// next: the next HTTP handler in the chain
func NewAdapterSelectorMiddleware(registry adapters.Registry, next http.Handler) http.Handler {
	return &AdapterSelectorMiddleware{
		registry: registry,
		next:     next,
	}
}

// ServeHTTP implements http.Handler, selecting the adapter for the requested database.
func (m *AdapterSelectorMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If no registry (single-database mode), pass through
	if m.registry == nil {
		m.next.ServeHTTP(w, r)
		return
	}

	// Extract database name from URL path (format: /{database}/...)
	// The router should set the database in the URL vars
	database := ""
	if routeVars := getRouteVars(r); routeVars != nil {
		database = routeVars["database"]
	}

	// If database name found, look up and attach adapter to context
	if database != "" {
		adapter, err := m.registry.Get(database)
		if err == nil {
			// Attach adapter to request context for handlers to use
			ctx := context.WithValue(r.Context(), pctx.AdapterKey, adapter)
			r = r.WithContext(ctx)
		}
		// If adapter not found, let handler deal with it (will get error from registry)
	}

	m.next.ServeHTTP(w, r)
}

// getRouteVars extracts route variables from the request context.
// This is a helper that works with gorilla/mux.
func getRouteVars(r *http.Request) map[string]string {
	// Try to get vars from context (set by mux)
	if vars, ok := r.Context().Value("mux.Vars").(map[string]string); ok {
		return vars
	}
	return nil
}
