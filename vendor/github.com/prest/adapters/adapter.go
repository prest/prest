package adapters

import (
	"net/http"
	"net/url"
)

//Adapter interface
type Adapter interface {
	TablePermissions(table string, op string) bool
	GetScript(verb, folder, scriptName string) (script string, err error)
	ParseScript(scriptPath string, queryURL url.Values) (sqlQuery string, values []interface{}, err error)
	ExecuteScripts(method, sql string, values []interface{}) (sc Scanner)
	WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error)
	DatabaseClause(req *http.Request) (query string, hasCount bool)
	OrderByRequest(r *http.Request) (values string, err error)
	PaginateIfPossible(r *http.Request) (paginatedQuery string, err error)
	Query(SQL string, params ...interface{}) (sc Scanner)
	SchemaClause(req *http.Request) (query string, hasCount bool)
	FieldsPermissions(r *http.Request, table string, op string) (fields []string, err error)
	SelectFields(fields []string) (sql string, err error)
	CountByRequest(req *http.Request) (countQuery string, err error)
	JoinByRequest(r *http.Request) (values []string, err error)
	GroupByClause(r *http.Request) (groupBySQL string)
	QueryCount(SQL string, params ...interface{}) (sc Scanner)
	ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error)
	Insert(SQL string, params ...interface{}) (sc Scanner)
	Delete(SQL string, params ...interface{}) (sc Scanner)
	SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error)
	Update(SQL string, params ...interface{}) (sc Scanner)
	DistinctClause(r *http.Request) (distinctQuery string, err error)
	SetDatabase(name string)
}
