package postgres

import (
	"context"
	"fmt"

	"github.com/prest/prest/v2/adapters/queryplan"
)

// ExplainRow runs an EXPLAIN on the connection selected for ctx and returns its
// raw payload, satisfying queryplan.Explainer. dbFromCtx falls back to the
// default pool when the context names no database, which also covers the
// non-context planner path.
//
// The statement is intentionally not closed: Stmt.PrepareContext caches and owns
// prepared statements for the whole adapter (see pg.cache), so closing one here
// would invalidate it for every other caller. Every query path in this adapter
// follows the same convention.
func (adapter *postgres) ExplainRow(ctx context.Context, SQL string, params ...interface{}) ([]byte, error) {
	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("select database for query plan: %w", err)
	}
	stmt, err := adapter.PrepareContext(ctx, db, SQL)
	if err != nil {
		return nil, fmt.Errorf("prepare query plan statement: %w", err)
	}
	var raw []byte
	if err := stmt.QueryRowContext(ctx, params...).Scan(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// Explain returns the execution plan for SQL using the default connection.
func (adapter *postgres) Explain(SQL string, params ...interface{}) (*queryplan.Node, error) {
	return adapter.queryPlanner().Explain(SQL, params...)
}

// ExplainCtx returns the execution plan for SQL using the database named in ctx.
func (adapter *postgres) ExplainCtx(ctx context.Context, SQL string, params ...interface{}) (*queryplan.Node, error) {
	return adapter.queryPlanner().ExplainCtx(ctx, SQL, params...)
}

// queryPlanner returns the injected planner, falling back to the postgres one for
// adapters built directly in tests rather than through New.
func (adapter *postgres) queryPlanner() queryplan.Planner {
	if adapter.planner != nil {
		return adapter.planner
	}
	return queryplan.NewPostgres(adapter)
}
