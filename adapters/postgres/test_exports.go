package postgres

import (
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
)

// SetDBConnectForTest replaces sqlx.Connect for unit tests outside this package
// (e.g. app composition-root tests) and returns a restore function.
// Callers must not use t.Parallel().
func SetDBConnectForTest(fn func(driverName, dataSourceName string) (*sqlx.DB, error)) func() {
	return connection.SetDBConnectForTest(fn)
}
