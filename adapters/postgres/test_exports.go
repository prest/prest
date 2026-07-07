package postgres

import (
	"context"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
)

// ChkInvalidIdentifierExported exposes chkInvalidIdentifier for integration tests.
func ChkInvalidIdentifierExported(identifier ...string) bool {
	return chkInvalidIdentifier(identifier...)
}

// ColumnsByRequestExported exposes columnsByRequest for integration tests.
func ColumnsByRequestExported(r *http.Request) ([]string, error) {
	return columnsByRequest(r)
}

// FieldsByPermissionExported exposes fieldsByPermission for integration tests.
func FieldsByPermissionExported(a adapters.Adapter, database, schema, table, operation, userName string) []string {
	p, ok := a.(*postgres)
	if !ok {
		return nil
	}
	return p.fieldsByPermission(database, schema, table, operation, userName)
}

// ClearStmtExported clears the prepared statement cache for integration tests.
func ClearStmtExported(a adapters.Adapter) {
	p, ok := a.(*postgres)
	if !ok {
		return
	}
	p.ClearStmt()
}

// GetStmtExported returns the statement cache for integration tests.
func GetStmtExported(a adapters.Adapter) *Stmt {
	p, ok := a.(*postgres)
	if !ok {
		return nil
	}
	return p.GetStmt()
}

// WriteSQLExported runs a write SQL statement for integration tests.
func WriteSQLExported(a adapters.Adapter, sql string, values []interface{}) adapters.Scanner {
	p, ok := a.(*postgres)
	if !ok {
		return nil
	}
	return p.WriteSQL(sql, values)
}

// WriteSQLCtxExported runs a write SQL statement with context for integration tests.
func WriteSQLCtxExported(ctx context.Context, a adapters.Adapter, sql string, values []interface{}) adapters.Scanner {
	p, ok := a.(*postgres)
	if !ok {
		return nil
	}
	return p.WriteSQLCtx(ctx, sql, values)
}

func PrepareExported(a adapters.Adapter, db *sqlx.DB, sql string) error {
	p, ok := a.(*postgres)
	if !ok {
		return ErrNotPostgresAdapter
	}
	_, err := p.Prepare(db, sql)
	return err
}
