package postgres

import (
	"context"

	"github.com/prest/prest/v2/adapters/access"
)

// ScriptPermissions checks whether a user may execute a stored query script.
// ctx is reserved for future DB-backed permission checks; the current implementation is config-only.
func (adapter *postgres) ScriptPermissions(_ context.Context, databaseAlias, location, name, op, userName string) bool {
	return access.ScriptAllowed(adapter.cfg, databaseAlias, location, name, op, userName)
}
