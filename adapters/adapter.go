package adapters

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	slog "github.com/structy/log"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/adapters/scanner"
	"github.com/prest/prest/config"
)

// Adapter interface for database operations
type Adapter interface {
	// GetTransaction attempts to get a transaction from the db connection
	GetTransaction() (tx *sql.Tx, err error)
	// GetTransactionCtx attempts to get a transaction from the
	// context setted db name
	//
	// use the adapter.DBNameKey for setting
	GetTransactionCtx(ctx context.Context) (tx *sql.Tx, err error)
	// GetConnURI returns the connection URI
	GetConnURI(DBName string) string
	// GetConn returns the current used connection from the pool
	GetConn() (*sql.DB, error)
	// GetConnCtx(ctx context.Context) (*sql.DB, error)

	// AddDatabaseToConnPool adds a connection to the pool
	AddDatabaseToConnPool(name string, DB *sql.DB)
	// MustGetConn returns the current used connection from the pool or panics
	MustGetConn() *sql.DB
	// SetCurrentConnDatabase sets the current connection database
	SetCurrentConnDatabase(name string)
	// GetCurrentConnDatabase returns the current connection database
	GetCurrentConnDatabase() string

	// BatchInsertValues execute batch insert sql into a table unsing params values
	BatchInsertValues(SQL string, params ...interface{}) (sc scanner.Scanner)
	BatchInsertValuesCtx(ctx context.Context, SQL string, params ...interface{}) (sc scanner.Scanner)
	// BatchInsertCopy executes a batch insert sql into a table unsing copy and given params
	BatchInsertCopy(dbname, schema, table string, keys []string, params ...interface{}) (sc scanner.Scanner)
	BatchInsertCopyCtx(ctx context.Context, dbname, schema, table string, keys []string, params ...interface{}) (sc scanner.Scanner)

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

	Delete(SQL string, params ...interface{}) (sc scanner.Scanner)
	DeleteCtx(ctx context.Context, SQL string, params ...interface{}) (sc scanner.Scanner)

	DeleteWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc scanner.Scanner)
	DeleteSQL(database string, schema string, table string) string
	DistinctClause(r *http.Request) (distinctQuery string, err error)

	ExecuteScripts(method, sql string, values []interface{}) (sc scanner.Scanner)
	ExecuteScriptsCtx(ctx context.Context, method, sql string, values []interface{}) (sc scanner.Scanner)

	FieldsPermissions(r *http.Request, table string, op string) (fields []string, err error)
	GetScript(verb, folder, scriptName string) (script string, err error)
	GroupByClause(r *http.Request) (groupBySQL string)

	Insert(SQL string, params ...interface{}) (sc scanner.Scanner)
	InsertCtx(ctx context.Context, SQL string, params ...interface{}) (sc scanner.Scanner)

	InsertWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc scanner.Scanner)
	InsertSQL(database string, schema string, table string, names string, placeholders string) string
	JoinByRequest(r *http.Request) (values []string, err error)
	OrderByRequest(r *http.Request) (values string, err error)
	PaginateIfPossible(r *http.Request) (paginatedQuery string, err error)
	ParseBatchInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
	ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
	ParseScript(scriptPath string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error)

	Query(SQL string, params ...interface{}) (sc scanner.Scanner)
	QueryCtx(ctx context.Context, SQL string, params ...interface{}) (sc scanner.Scanner)
	QueryCount(SQL string, params ...interface{}) (sc scanner.Scanner)
	QueryCountCtx(ctx context.Context, SQL string, params ...interface{}) (sc scanner.Scanner)

	ReturningByRequest(r *http.Request) (returningSyntax string, err error)
	SchemaClause(req *http.Request) (query string, hasCount bool)
	SchemaOrderBy(order string, hasCount bool) (orderBy string)
	SchemaTablesClause() (query string)
	SchemaTablesOrderBy(order string) (orderBy string)
	SchemaTablesWhere(requestWhere string) (whereSyntax string)
	SelectFields(fields []string) (sql string, err error)
	SelectSQL(selectStr string, database string, schema string, table string) string
	SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error)

	TableClause() (query string)
	TableOrderBy(order string) (orderBy string)
	TablePermissions(table string, op string) bool
	TableWhere(requestWhere string) (whereSyntax string)

	Update(SQL string, params ...interface{}) (sc scanner.Scanner)
	UpdateCtx(ctx context.Context, SQL string, params ...interface{}) (sc scanner.Scanner)

	UpdateWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc scanner.Scanner)
	UpdateSQL(database string, schema string, table string, setSyntax string) string
	WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error)

	ShowTable(schema, table string) (sc scanner.Scanner)
	ShowTableCtx(ctx context.Context, schema, table string) (sc scanner.Scanner)
}

var (
	// ErrAdapterNotSupported is returned when the adapter is not supported
	ErrAdapterNotSupported = fmt.Errorf("adapter not supported")
)

// New returns a new adapter based on the configuration file
//
// currently only postgres is supported
// TODO: add support to multiple adapters
func New(cfg *config.Prest) (Adapter, error) {
	switch cfg.Adapter {
	case "postgres":
		return postgres.NewAdapter(cfg), nil
	case "":
		slog.Warningln("no adapter defined, using postgres")
		return postgres.NewAdapter(cfg), nil
	}
	return nil, ErrAdapterNotSupported
}
