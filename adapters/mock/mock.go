package mock

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/scanner"
	"github.com/prest/prest/v2/config"
)

// Item mock
type Item struct {
	Body    []byte
	Error   error
	IsCount bool
}

// Mock adapter
type Mock struct {
	mtx   *sync.RWMutex
	t     *testing.T
	conns map[string]*mockConn
	Items []Item
}

var _ adapters.Adapter = (*Mock)(nil) // Verify that Mock implements Adapter.

// New mock
func New(t *testing.T) (m *Mock) {
	m = &Mock{
		mtx: &sync.RWMutex{},
		t:   t,
	}
	drivers := sql.Drivers()
	for _, driver := range drivers {
		if driver == "mock" {
			return
		}
	}
	sql.Register("mock", m)
	return
}

// Open makes Mock implement driver.Driver
func (m *Mock) Open(dsn string) (c driver.Conn, err error) {
	m.t.Helper()
	m.conns = make(map[string]*mockConn)
	m.conns["prest"] = &mockConn{}
	c, ok := m.conns[dsn]
	if !ok {
		slog.Debug(
			"mock connection not found",
			"dsn", dsn,
			"available_dsns", maps.Keys(m.conns),
		)
		return c, fmt.Errorf("expected a connection to be available, but it is not: conn=%v, available_dsns=%v", c, maps.Keys(m.conns))
	}
	return
}

func (m *Mock) validate() {
	m.t.Helper()
	if len(m.Items) == 0 {
		m.t.Fatal("do not have any operations to perform")
	}
}

func (m *Mock) perform(query bool) (sc adapters.Scanner) {
	m.t.Helper()
	m.validate()
	m.mtx.Lock()
	item := m.Items[0]
	sc = &scanner.PrestScanner{
		Error:   item.Error,
		Buff:    bytes.NewBuffer(item.Body),
		IsQuery: query,
	}
	m.Items = m.Items[1:]
	m.mtx.Unlock()
	return
}

// TablePermissions mock
func (m *Mock) TablePermissions(table string, op string, userName string) (ok bool) {
	m.t.Helper()
	restrict := config.PrestConf.AccessConf.Restrict
	if !restrict {
		return true
	}

	tables := config.PrestConf.AccessConf.Tables
	access := false
	for _, t := range tables {
		if t.Name == table {
			access = slices.Contains(t.Permissions, op)
			break
		}
	}

	// If userName is empty, means use table access.
	if userName == "" {
		return access
	}

	// currently, access is granted to all users based on the table settings.
	// if it is later discovered that there are specific permission settings for an individual user,
	// then the latter settings should be applied.
	users := config.PrestConf.AccessConf.Users
	for _, u := range users {
		if u.Name == userName {
			for _, t := range u.Tables {
				if t.Name == table {
					return slices.Contains(t.Permissions, op)
				}
			}
		}
	}
	return access
}

// GetScript mock
func (m *Mock) GetScript(verb string, folder string, scriptName string) (script string, err error) {
	return
}

// ParseScript mock
func (m *Mock) ParseScript(scriptPath string, data map[string]interface{}) (sqlQuery string, values []interface{}, err error) {
	return
}

// ExecuteScripts mock
func (m *Mock) ExecuteScripts(method string, sql string, values []interface{}) (sc adapters.Scanner) {
	return
}

// ExecuteScripts mock
func (m *Mock) ExecuteScriptsCtx(ctx context.Context, method string, sql string, values []interface{}) (sc adapters.Scanner) {
	return
}

// WhereByRequest mock
func (m *Mock) WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error) {
	return
}

// ReturningByRequest mock
func (m *Mock) ReturningByRequest(r *http.Request) (ReturningSyntax string, err error) {
	return
}

// DatabaseClause mock
func (m *Mock) DatabaseClause(req *http.Request) (query string, hasCount bool) {
	m.t.Helper()
	m.validate()
	m.mtx.Lock()
	hasCount = m.Items[0].IsCount
	m.mtx.Unlock()
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

// GetTransaction mock
func (m *Mock) GetTransaction() (tx *sql.Tx, err error) {
	db, err := sql.Open("mock", "prest")
	if err != nil {
		return
	}
	return db.Begin()
}

// GetTransactionCtx mock
func (m *Mock) GetTransactionCtx(ctx context.Context) (tx *sql.Tx, err error) {
	db, err := sql.Open("mock", "prest")
	if err != nil {
		return
	}
	return db.Begin()
}

// Query mock
func (m *Mock) Query(SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(true)
	return
}

// QueryCtx mock
func (m *Mock) QueryCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(true)
	return
}

// SchemaClause mock
func (m *Mock) SchemaClause(req *http.Request) (query string, hasCount bool) {
	m.t.Helper()
	m.validate()
	m.mtx.Lock()
	hasCount = m.Items[0].IsCount
	m.mtx.Unlock()
	return
}

// FieldsPermissions mock
func (m *Mock) FieldsPermissions(r *http.Request, table string, op string, userName string) (fields []string, err error) {
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
	m.t.Helper()
	sc = m.perform(false)
	return
}

// QueryCountCtx mock
func (m *Mock) QueryCountCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// ParseInsertRequest mock
func (m *Mock) ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error) {
	return
}

// Insert mock
func (m *Mock) Insert(SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// Insert mock
func (m *Mock) InsertCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// InsertWithTransaction mock
func (m *Mock) InsertWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// Delete mock
func (m *Mock) Delete(SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// DeleteCtx mock
func (m *Mock) DeleteCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// DeleteWithTransaction mock
func (m *Mock) DeleteWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// SetByRequest mock
func (m *Mock) SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error) {
	return
}

// Update mock
func (m *Mock) Update(SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// UpdateCtx mock
func (m *Mock) UpdateCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// UpdateWithTransaction mock
func (m *Mock) UpdateWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
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

// ParseBatchInsertRequest mocl
func (m *Mock) ParseBatchInsertRequest(r *http.Request) (colsName string, placeholders string, values []interface{}, err error) {
	return
}

// BatchInsertValues mock
func (m *Mock) BatchInsertValues(SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(true)
	return
}

// BatchInsertValuesCtx mock
func (m *Mock) BatchInsertValuesCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(true)
	return
}

// BatchInsertCopy mock
func (m *Mock) BatchInsertCopy(dbname, schema, table string, keys []string, values ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// BatchInsertCopyCtx mock
func (m *Mock) BatchInsertCopyCtx(ctx context.Context, dbname, schema, table string, keys []string, values ...interface{}) (sc adapters.Scanner) {
	m.t.Helper()
	sc = m.perform(false)
	return
}

// ShowTable shows table structure
func (m *Mock) ShowTable(schema, table string) (sc adapters.Scanner) {
	return
}

// ShowTableCtx shows table structure
func (m *Mock) ShowTableCtx(ctx context.Context, schema, table string) (sc adapters.Scanner) {
	return
}

// AddItem on mock object
func (m *Mock) AddItem(body []byte, err error, isCount bool) {
	i := Item{
		Body:    body,
		Error:   err,
		IsCount: isCount,
	}
	m.mtx.Lock()
	m.Items = append(m.Items, i)
	m.mtx.Unlock()
}

// GetDatabase ron mock db
func (m *Mock) GetDatabase() (db string) {
	return
}
