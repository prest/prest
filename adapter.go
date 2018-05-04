package adapters

import (
	"net/http"
	"net/url"
)

//Adapter interface
type Adapter interface {
	BatchInsertValues(SQL string, params ...interface{}) (sc Scanner)
	BatchInsertCopy(dbname, schema, table string, keys []string, params ...interface{}) (sc Scanner)
	CountByRequest(req *http.Request) (countQuery string, err error)
	DatabaseClause(req *http.Request) (query string, hasCount bool)
	DatabaseOrderBy(order string, hasCount bool) (orderBy string)
	DatabaseWhere(requestWhere string) (whereSyntax string)
	Delete(SQL string, params ...interface{}) (sc Scanner)
	DeleteSQL(database string, schema string, table string) string
	DistinctClause(r *http.Request) (distinctQuery string, err error)
	ExecuteScripts(method, sql string, values []interface{}) (sc Scanner)
	FieldsPermissions(r *http.Request, table string, op string) (fields []string, err error)
	GetScript(verb, folder, scriptName string) (script string, err error)
	GroupByClause(r *http.Request) (groupBySQL string)
	Insert(SQL string, params ...interface{}) (sc Scanner)
	InsertSQL(database string, schema string, table string, names string, placeholders string) string
	JoinByRequest(r *http.Request) (values []string, err error)
	OrderByRequest(r *http.Request) (values string, err error)
	PaginateIfPossible(r *http.Request) (paginatedQuery string, err error)
	ParseBatchInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
	ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
	ParseScript(scriptPath string, queryURL url.Values) (sqlQuery string, values []interface{}, err error)
	Query(SQL string, params ...interface{}) (sc Scanner)
	QueryCount(SQL string, params ...interface{}) (sc Scanner)
	SchemaClause(req *http.Request) (query string, hasCount bool)
	SchemaOrderBy(order string, hasCount bool) (orderBy string)
	SchemaTablesClause() (query string)
	SchemaTablesOrderBy(order string) (orderBy string)
	SchemaTablesWhere(requestWhere string) (whereSyntax string)
	SelectFields(fields []string) (sql string, err error)
	SelectSQL(selectStr string, database string, schema string, table string) string
	SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error)
	SetDatabase(name string)
	TableClause() (query string)
	TableOrderBy(order string) (orderBy string)
	TablePermissions(table string, op string) bool
	TableWhere(requestWhere string) (whereSyntax string)
	Update(SQL string, params ...interface{}) (sc Scanner)
	UpdateSQL(database string, schema string, table string, setSyntax string) string
	WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error)
}
