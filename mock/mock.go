package mock

import (
	"bytes"
	"net/http"
	"net/url"

	"github.com/prest/adapters"
	"github.com/prest/adapters/internal/scanner"
)

// Item mock
type Item struct {
	Body          []byte
	Error         error
	HasPermission bool
	IsCount       bool
}

// Mock adapter
type Mock struct {
	Items []Item
}

func (m *Mock) validate() {
	if len(m.Items) == 0 {
		panic("do not have any operations to perform")
	}
}

func (m *Mock) perform() (sc adapters.Scanner) {
	m.validate()
	item := m.Items[0]
	sc = &scanner.PrestScanner{
		Error: item.Error,
		Buff:  bytes.NewBuffer(item.Body),
	}
	m.Items = m.Items[1:]
	return
}

// TablePermissions mock
func (m *Mock) TablePermissions(table string, op string) bool {
	m.validate()
	return m.Items[0].HasPermission
}

// GetScript mock
func (m *Mock) GetScript(verb string, folder string, scriptName string) (script string, err error) {
	return
}

// ParseScript mock
func (m *Mock) ParseScript(scriptPath string, queryURL url.Values) (sqlQuery string, values []interface{}, err error) {
	return
}

// ExecuteScripts mock
func (m *Mock) ExecuteScripts(method string, sql string, values []interface{}) (sc adapters.Scanner) {
	return
}

// WhereByRequest mock
func (m *Mock) WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error) {
	return
}

// DatabaseClause mock
func (m *Mock) DatabaseClause(req *http.Request) (query string, hasCount bool) {
	m.validate()
	hasCount = m.Items[0].IsCount
	return
}

// OrderByRequest mock
func (m *Mock) OrderByRequest(r *http.Request) (values string, err error) {
	return
}

// PaginateIfPossible mock
func (m *Mock) PaginateIfPossible(r *http.Request) (paginatedQuery string, err error) {
	return
}

// Query mock
func (m *Mock) Query(SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.perform()
	return
}

// SchemaClause mock
func (m *Mock) SchemaClause(req *http.Request) (query string, hasCount bool) {
	m.validate()
	hasCount = m.Items[0].IsCount
	return
}

// FieldsPermissions mock
func (m *Mock) FieldsPermissions(r *http.Request, table string, op string) (fields []string, err error) {
	fields = append(fields, "mock")
	return
}

// SelectFields mock
func (m *Mock) SelectFields(fields []string) (sql string, err error) {
	return
}

// CountByRequest mock
func (m *Mock) CountByRequest(req *http.Request) (countQuery string, err error) {
	return
}

// JoinByRequest mock
func (m *Mock) JoinByRequest(r *http.Request) (values []string, err error) {
	return
}

// GroupByClause mock
func (m *Mock) GroupByClause(r *http.Request) (groupBySQL string) {
	return
}

// QueryCount mock
func (m *Mock) QueryCount(SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.perform()
	return
}

// ParseInsertRequest mock
func (m *Mock) ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error) {
	return
}

// Insert mock
func (m *Mock) Insert(SQL string, params ...interface{}) (sc adapters.Scanner) {
	sc = m.perform()
	return
}

// Delete mock
func (m *Mock) Delete(SQL string, params ...interface{}) (sc adapters.Scanner) {
	sc = m.perform()
	return
}

// SetByRequest mock
func (m *Mock) SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error) {
	return
}

// Update mock
func (m *Mock) Update(SQL string, params ...interface{}) (sc adapters.Scanner) {
	sc = m.perform()
	return
}

// DistinctClause mock
func (m *Mock) DistinctClause(r *http.Request) (distinctQuery string, err error) {
	return
}

// SetDatabase mock
func (m *Mock) SetDatabase(name string) {
}

// SelectSQL mock
func (m *Mock) SelectSQL(selectStr string, database string, schema string, table string) (s string) {
	return
}

// InsertSQL mock
func (m *Mock) InsertSQL(database string, schema string, table string, names string, placeholders string) (s string) {
	return
}

// DeleteSQL mock
func (m *Mock) DeleteSQL(database string, schema string, table string) (s string) {
	return
}

// UpdateSQL mock
func (m *Mock) UpdateSQL(database string, schema string, table string, setSyntax string) (s string) {
	return
}

// DatabaseWhere mock
func (m *Mock) DatabaseWhere(requestWhere string) (whereSyntax string) {
	return
}

// DatabaseOrderBy mock
func (m *Mock) DatabaseOrderBy(order string, hasCount bool) (orderBy string) {
	return
}

// SchemaOrderBy mock
func (m *Mock) SchemaOrderBy(order string, hasCount bool) (orderBy string) {
	return
}

// TableClause mock
func (m *Mock) TableClause() (query string) {
	return
}

// TableWhere mock
func (m *Mock) TableWhere(requestWhere string) (whereSyntax string) {
	return
}

// TableOrderBy mock
func (m *Mock) TableOrderBy(order string) (orderBy string) {
	return
}

// SchemaTablesClause mock
func (m *Mock) SchemaTablesClause() (query string) {
	return
}

// SchemaTablesWhere mock
func (m *Mock) SchemaTablesWhere(requestWhere string) (whereSyntax string) {
	return
}

// SchemaTablesOrderBy mock
func (m *Mock) SchemaTablesOrderBy(order string) (orderBy string) {
	return
}
