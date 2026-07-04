package adapters

import (
	"context"
	"database/sql"
)

// TransactionManager provides database transactions.
type TransactionManager interface {
	GetTransaction() (tx *sql.Tx, err error)
	GetTransactionCtx(ctx context.Context) (tx *sql.Tx, err error)
}

// LegacyExecutor provides non-context execution with transaction support.
type LegacyExecutor interface {
	InsertWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc Scanner)
	UpdateWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc Scanner)
	DeleteWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc Scanner)
}

// Adapter is the composite interface implemented by database adapters.
//
// Connection lifecycle (Connect, DB, Ping) is intentionally not part of Adapter.
// It lives on DatabaseConnector, DatabaseAccessor, and DatabasePinger so test
// doubles can implement Adapter without a real database. Production postgres
// adapters also satisfy those interfaces; use type assertions where needed.
type Adapter interface {
	RequestQueryBuilder
	QueryExecutor
	CatalogQuerier
	SQLBuilder
	PermissionsChecker
	ScriptRunner
	DatabaseRegistry
	TransactionManager
	LegacyExecutor
}
