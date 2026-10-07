package adapters

import "context"

// SystemTableEnsurer creates engine-specific auth and prest_queries tables.
//
// Optional capability: not embedded in Adapter. Startup type-asserts the
// configured adapter and, when this interface is implemented, uses it instead
// of the Postgres DDL in app/schema.go.
type SystemTableEnsurer interface {
	EnsureAuthTable(ctx context.Context) error
	EnsureQueriesTable(ctx context.Context) error
}
