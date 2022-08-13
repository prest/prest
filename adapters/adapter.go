package adapters

import (
	"context"
	"database/sql"
	"net/http"
)

//Adapter interface
type Adapter interface {
	// GetTransaction attempts to get a transaction from the db connection
	GetTransaction() (tx *sql.Tx, err error)

	// BatchInsertValues execute batch insert sql into a table unsing params values
	BatchInsertValues(SQL string, params ...interface{}) (sc Scanner)
	BatchInsertValuesCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)

	// BatchInsertCopy executes a batch insert sql into a table unsing copy and given params
	BatchInsertCopy(dbname, schema, table string, keys []string, params ...interface{}) (sc Scanner)
	BatchInsertCopyCtx(ctx context.Context, dbname, schema, table string, keys []string, params ...interface{}) (sc Scanner)

	// CountByRequest implements COUNT(fields) OPERTATION
	//
	// returns a `SELECT COUNT(%s) FROM` query with the given
	// URL query values from named '_count' key
	CountByRequest(req *http.Request) (countQuery string, err error)

	// DatabaseClause returns a SELECT from URL query params
	//
	// returns 'SELECT (datname|COUNT(datname)) FROM pg_database'
	// if no `_count` query param provided will return the first option above
	DatabaseClause(req *http.Request) (query string, hasCount bool)

	// DatabaseOrderBy generate database order by statement
	//
	// if order argument is empty and hasCount=true, will return empty string
	DatabaseOrderBy(order string, hasCount bool) (orderBy string)

	// DatabaseWhere generates a database where syntax
	//
	// returns 'WHERE NOT datistemplate' if requestWhere is not provided
	//
	// returns 'WHERE NOT datistemplate AND <requestWhere>' if provided
	DatabaseWhere(requestWhere string) (whereSyntax string)
	Delete(SQL string, params ...interface{}) (sc Scanner)
	DeleteWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc Scanner)
	DeleteSQL(database string, schema string, table string) string
	DistinctClause(r *http.Request) (distinctQuery string, err error)
	ExecuteScripts(method, sql string, values []interface{}) (sc Scanner)
	FieldsPermissions(r *http.Request, table string, op string) (fields []string, err error)
	GetScript(verb, folder, scriptName string) (script string, err error)
	GroupByClause(r *http.Request) (groupBySQL string)

	Insert(SQL string, params ...interface{}) (sc Scanner)
	InsertCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)

	InsertWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc Scanner)
	InsertSQL(database string, schema string, table string, names string, placeholders string) string
	JoinByRequest(r *http.Request) (values []string, err error)
	OrderByRequest(r *http.Request) (values string, err error)
	PaginateIfPossible(r *http.Request) (paginatedQuery string, err error)
	ParseBatchInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
	ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
	ParseScript(scriptPath string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error)

	Query(SQL string, params ...interface{}) (sc Scanner)
	QueryCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)
	QueryCount(SQL string, params ...interface{}) (sc Scanner)
	QueryCountCtx(ctx context.Context, SQL string, params ...interface{}) (sc Scanner)

	ReturningByRequest(r *http.Request) (returningSyntax string, err error)
	SchemaClause(req *http.Request) (query string, hasCount bool)
	SchemaOrderBy(order string, hasCount bool) (orderBy string)
	SchemaTablesClause() (query string)
	SchemaTablesOrderBy(order string) (orderBy string)
	SchemaTablesWhere(requestWhere string) (whereSyntax string)
	SelectFields(fields []string) (sql string, err error)
	SelectSQL(selectStr string, database string, schema string, table string) string
	SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error)
	SetDatabase(name string)
	GetDatabase() string
	TableClause() (query string)
	TableOrderBy(order string) (orderBy string)
	TablePermissions(table string, op string) bool
	TableWhere(requestWhere string) (whereSyntax string)
	Update(SQL string, params ...interface{}) (sc Scanner)
	UpdateWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc Scanner)
	UpdateSQL(database string, schema string, table string, setSyntax string) string
	WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error)

	ShowTable(schema, table string) (sc Scanner)
	ShowTableCtx(ctx context.Context, schema, table string) (sc Scanner)
}
