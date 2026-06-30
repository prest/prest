package adapters

import "context"

// QueryExecutor runs SQL statements against the database.
type QueryExecutor interface {
	Query(SQL string, params ...interface{}) (sc Scanner)
	QueryCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)
	QueryCount(SQL string, params ...interface{}) (sc Scanner)
	QueryCountCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)

	Insert(SQL string, params ...interface{}) (sc Scanner)
	InsertCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)

	Update(SQL string, params ...interface{}) (sc Scanner)
	UpdateCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)

	Delete(SQL string, params ...interface{}) (sc Scanner)
	DeleteCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)

	BatchInsertValues(SQL string, params ...interface{}) (sc Scanner)
	BatchInsertValuesCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)

	BatchInsertCopy(dbname, schema, table string, keys []string, params ...interface{}) (sc Scanner)
	BatchInsertCopyCtx(ctx context.Context, dbname, schema, table string, keys []string, params ...interface{}) (sc Scanner)

	ShowTable(schema, table string) (sc Scanner)
	ShowTableCtx(ctx context.Context, schema, table string) (sc Scanner)

	ExecuteScripts(method, sql string, values []interface{}) (sc Scanner)
	ExecuteScriptsCtx(ctx context.Context, method, sql string, values []interface{}) (sc Scanner)
}
