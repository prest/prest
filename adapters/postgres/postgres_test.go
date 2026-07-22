package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
	"github.com/prest/prest/v2/adapters/postgres/statements"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

const (
	defaultMockDB = "default-db"
	contextMockDB = "ctx-db"
)

func testAdapter(cfg ...*config.Prest) *postgres {
	c := defaultTestConf()
	if len(cfg) > 0 && cfg[0] != nil {
		c = cfg[0]
	}
	return New(c).(*postgres)
}

func defaultTestConf() *config.Prest {
	return &config.Prest{
		PGDatabase:  defaultMockDB,
		JSONAggType: "json_agg",
		PGCache:     false,
		PGHost:      "localhost",
		PGPort:      5432,
		PGUser:      "u",
		PGSSLMode:   "disable",
	}
}

// withFailingDBConnect stubs sqlx.Connect via a package-level hook. Callers must
// not use t.Parallel(); see agentic-loop serial test rules for shared globals.
func withFailingDBConnect(t *testing.T, msg string) *postgres {
	t.Helper()
	restore := connection.SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, errors.New(msg)
	})
	t.Cleanup(restore)
	return New(defaultTestConf()).(*postgres)
}

func withSQLMock(t *testing.T) (*postgres, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	cfg := defaultTestConf()
	pg := New(cfg).(*postgres)
	pg.conn.SetDatabase(defaultMockDB)
	pg.conn.InjectDBForTest(pg.conn.GetURI(defaultMockDB), sqlxDB)
	t.Cleanup(func() { pg.conn.ResetPoolForTest() })
	pg.ClearStmt()
	t.Cleanup(pg.ClearStmt)

	return pg, mock
}

func withSQLMocks(t *testing.T) (*postgres, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()
	defaultDB, defaultMock, err := sqlmock.New()
	require.NoError(t, err)
	ctxDB, ctxMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = defaultDB.Close()
		_ = ctxDB.Close()
	})

	cfg := defaultTestConf()
	pg := New(cfg).(*postgres)
	pg.conn.SetDatabase(defaultMockDB)
	pg.conn.InjectDBForTest(pg.conn.GetURI(defaultMockDB), sqlx.NewDb(defaultDB, "sqlmock"))
	pg.conn.InjectDBForTest(pg.conn.GetURI(contextMockDB), sqlx.NewDb(ctxDB, "sqlmock"))
	t.Cleanup(func() { pg.conn.ResetPoolForTest() })
	pg.ClearStmt()
	t.Cleanup(pg.ClearStmt)

	return pg, defaultMock, ctxMock
}

func withSQLMockPing(t *testing.T) (*postgres, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	cfg := defaultTestConf()
	pg := New(cfg).(*postgres)
	pg.conn.SetDatabase(defaultMockDB)
	pg.conn.InjectDBForTest(pg.conn.GetURI(defaultMockDB), sqlxDB)
	t.Cleanup(func() { pg.conn.ResetPoolForTest() })
	pg.ClearStmt()
	t.Cleanup(pg.ClearStmt)

	return pg, mock
}

func registryTestConf(aliases ...string) *config.Prest {
	cfg := defaultTestConf()
	for _, alias := range aliases {
		cfg.Databases = append(cfg.Databases, config.DatabaseConf{
			Alias:    alias,
			Database: alias + "_db",
		})
	}
	return cfg
}

type errRowsAffectedResult struct{}

func (errRowsAffectedResult) LastInsertId() (int64, error) { return 0, nil }
func (errRowsAffectedResult) RowsAffected() (int64, error) {
	return 0, errors.New("rows affected failed")
}

func permissionTestConf() *config.Prest {
	cfg := defaultTestConf()
	cfg.AccessConf = config.AccessConf{
		Restrict:    true,
		IgnoreTable: []string{"ignored_table"},
		Tables: []config.TablesConf{
			{Name: "test_readonly_access", Permissions: []string{"read"}, Fields: []string{"*"}},
			{Name: "test_write_and_delete_access", Permissions: []string{"write", "delete"}, Fields: []string{"name", "surname"}},
			{Name: "test_fields_access", Permissions: []string{"read"}, Fields: []string{"name", "surname"}},
			{Name: "test_permission_does_not_exist", Permissions: []string{}, Fields: []string{"*"}},
		},
		Users: []config.UsersConf{
			{
				Name: "foo_read",
				Tables: []config.TablesConf{
					{Name: "no_user_write_table", Permissions: []string{"write"}, Fields: []string{"name"}},
					{Name: "no_user_delete_table", Permissions: []string{"delete"}, Fields: []string{"id"}},
				},
			},
		},
	}
	return cfg
}

func TestGetQueryOperator(t *testing.T) {

	t.Parallel()

	testCases := []struct {
		in  string
		out string
	}{
		{"$eq", "="},
		{"$ne", "!="},
		{"$gt", ">"},
		{"$gte", ">="},
		{"$lt", "<"},
		{"$lte", "<="},
		{"$in", "IN"},
		{"$nin", "NOT IN"},
		{"$any", "ANY"},
		{"$some", "SOME"},
		{"$all", "ALL"},
		{"$notnull", "IS NOT NULL"},
		{"$null", "IS NULL"},
		{"$true", "IS TRUE"},
		{"$nottrue", "IS NOT TRUE"},
		{"$false", "IS FALSE"},
		{"$notfalse", "IS NOT FALSE"},
		{"$like", "LIKE"},
		{"$ilike", "ILIKE"},
		{"$nlike", "NOT LIKE"},
		{"$nilike", "NOT ILIKE"},
		{"$ltreelanc", "@>"},
		{"$ltreerdesc", "<@"},
		{"$ltreematch", "~"},
		{"$ltreematchtxt", "@"},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			op, err := GetQueryOperator(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.out, op)
		})
	}

	op, err := GetQueryOperator("!lol")
	require.Error(t, err)
	require.Empty(t, op)
	require.ErrorIs(t, err, ErrInvalidOperator)
}

func TestChkInvalidIdentifier(t *testing.T) {

	t.Parallel()

	testCases := []struct {
		in  string
		out bool
	}{
		{"fildName", false},
		{"_9fildName", false},
		{"_fild.Name", false},
		{"0fildName", true},
		{"fild'Name", true},
		{"fild\"Name", true},
		{"fild;Name", true},
		{"SUM(test)", false},
		{`SUM("test")`, false},
		{"_123456789_123456789_123456789_123456789_123456789_123456789_12345", true},
		{"not_invalid_table.with_not_invalid_column", false},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.out, chkInvalidIdentifier(tc.in))
		})
	}
}

func TestNormalizeGroupFunction(t *testing.T) {

	t.Parallel()

	testCases := []struct {
		urlValue    string
		expectedSQL string
	}{
		{"avg:age", `AVG("age")`},
		{"sum:age", `SUM("age")`},
		{"max:age", `MAX("age")`},
		{"min:age", `MIN("age")`},
		{"stddev:age", `STDDEV("age")`},
		{"variance:age", `VARIANCE("age")`},
		{"avg:age:colname", `AVG("age") AS "colname"`},
	}

	for _, tc := range testCases {
		t.Run(tc.urlValue, func(t *testing.T) {
			sql, err := NormalizeGroupFunction(tc.urlValue)
			require.NoError(t, err)
			require.Equal(t, tc.expectedSQL, sql)
		})
	}

	_, err := NormalizeGroupFunction("invalid:age")
	require.Error(t, err)
}

func TestWhereByRequest(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	testCases := []struct {
		name           string
		url            string
		expectedSQL    []string
		expectedValues []string
		expectNoWhere  bool
		expectNoValues bool
		err            error
	}{
		{
			name:           "basic equality",
			url:            "/databases?dbname=$eq.prest&test=$eq.cool",
			expectedSQL:    []string{`"dbname" = $`, `"test" = $`, " AND "},
			expectedValues: []string{"prest", "cool"},
		},
		{
			name:           "with alias",
			url:            "/databases?dbname=$eq.prest&c.test=$eq.cool",
			expectedSQL:    []string{`"dbname" = $`, `"c".`, `"test" = $`, " AND "},
			expectedValues: []string{"prest", "cool"},
		},
		{
			name:           "with like",
			url:            "/public/test5?name=$like.%25val%25&phonenumber=123456",
			expectedSQL:    []string{`"name" LIKE $`, `"phonenumber" = $`, " AND "},
			expectedValues: []string{"%val%", "123456"},
		},
		{
			name:           "with ilike",
			url:            "/public/test5?name=$ilike.%25vAl%25&phonenumber=123456",
			expectedSQL:    []string{`"name" ILIKE $`, `"phonenumber" = $`, " AND "},
			expectedValues: []string{"%vAl%", "123456"},
		},
		{
			name:           "with jsonb field",
			url:            "/public/test_jsonb?name=$eq.goku&data->>description:jsonb=$eq.testing",
			expectedSQL:    []string{`"name" = $`, `"data"->>'description' = $`, " AND "},
			expectedValues: []string{"goku", "testing"},
		},
		{
			name:           "with _or",
			url:            "/public/test5?_or=name=$eq.prest||phoneNumber=$eq.123",
			expectedSQL:    []string{`("name" = $`, ` OR "phoneNumber" = $`, `)`},
			expectedValues: []string{"prest", "123"},
		},
		{
			name:           "with _or using $in",
			url:            "/public/test5?_or=name=$in.foo,bar||phoneNumber=$eq.123",
			expectedSQL:    []string{`("name" IN (`, ` OR "phoneNumber" = $`, `)`},
			expectedValues: []string{"foo", "bar", "123"},
		},
		{
			name:           "with ltree left acendent",
			url:            "/public/test5?path='$ltreelanc.Top.*'",
			expectedSQL:    []string{`"path" @> $`},
			expectedValues: []string{`'Top.*'`},
		},
		{
			name:           "with ltree match lquery",
			url:            "/public/test5?path='$ltreematch.Top.*'",
			expectedSQL:    []string{`"path" ~ $`},
			expectedValues: []string{`'Top.*'`},
		},
		{
			name:           "malformed _or",
			url:            "/public/test5?_or=namevalue",
			expectNoWhere:  true,
			expectNoValues: true,
		},
		{
			name:           "empty _or",
			url:            "/public/test5?_or=",
			expectNoWhere:  true,
			expectNoValues: true,
		},
		{
			name:           "invalid _or rhs",
			url:            "/public/test5?_or=name=",
			expectNoWhere:  true,
			expectNoValues: true,
			err:            ErrInvalidOperator,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)

			where, values, err := adapter.WhereByRequest(req, 1)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}

			if tc.expectNoWhere {
				require.Empty(t, where)
			} else {
				for _, fragment := range tc.expectedSQL {
					require.Contains(t, where, fragment)
				}
			}

			if tc.expectNoValues {
				require.Empty(t, values)
			} else {
				require.Len(t, values, len(tc.expectedValues))
				expected := strings.Join(tc.expectedValues, " ")
				for _, value := range values {
					require.Contains(t, expected, value.(string))
				}
			}
		})
	}
}

func TestSetByRequest(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	m := map[string]interface{}{"name": "prest"}
	mc := map[string]interface{}{"test": "prest", "dbname": "prest"}

	testCases := []struct {
		name           string
		body           map[string]interface{}
		expectedSQL    []string
		expectedValues []string
		err            error
	}{
		{
			name:           "multiple fields",
			body:           mc,
			expectedSQL:    []string{`"dbname"=$`, `"test"=$`, ", "},
			expectedValues: []string{"prest", "prest"},
		},
		{
			name:           "single field",
			body:           m,
			expectedSQL:    []string{`"name"=$`},
			expectedValues: []string{"prest"},
		},
		{
			name: "empty body",
			body: nil,
			err:  ErrBodyEmpty,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
			require.NoError(t, err)

			setSyntax, values, err := adapter.SetByRequest(req, 1)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			for _, fragment := range tc.expectedSQL {
				require.Contains(t, setSyntax, fragment)
			}
			expected := strings.Join(tc.expectedValues, " ")
			for _, value := range values {
				require.Contains(t, expected, value.(string))
			}
		})
	}
}

func TestSetByRequest_ValueTypes(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	t.Run("slice value", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{"tags": []interface{}{"a", "b"}})
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
		require.NoError(t, err)

		setSyntax, values, err := adapter.SetByRequest(req, 1)
		require.NoError(t, err)
		require.Contains(t, setSyntax, `"tags"=$1`)
		require.Equal(t, `["a", "b"]`, values[0])
	})

	t.Run("map value", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{"meta": map[string]interface{}{"k": "v"}})
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
		require.NoError(t, err)

		_, values, err := adapter.SetByRequest(req, 1)
		require.NoError(t, err)
		require.JSONEq(t, `{"k":"v"}`, values[0].(string))
	})

	t.Run("invalid identifier", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{"0bad": "x"})
		require.NoError(t, err)
		req, err := http.NewRequest(http.MethodPut, "/", bytes.NewReader(body))
		require.NoError(t, err)

		_, _, err = adapter.SetByRequest(req, 1)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidIdentifier)
	})

	t.Run("invalid json", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("{")))
		require.NoError(t, err)

		_, _, err = adapter.SetByRequest(req, 1)
		require.Error(t, err)
	})
}

func TestParseInsertRequest(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	m := map[string]interface{}{"name": "prest"}
	mc := map[string]interface{}{"test": "prest", "dbname": "prest"}

	testCases := []struct {
		name             string
		body             map[string]interface{}
		expectedColNames []string
		expectedValues   []string
		err              error
	}{
		{
			name:             "multiple fields",
			body:             mc,
			expectedColNames: []string{"dbname", "test"},
			expectedValues:   []string{"prest", "prest"},
		},
		{
			name:             "single field",
			body:             m,
			expectedColNames: []string{"name"},
			expectedValues:   []string{"prest"},
		},
		{
			name: "empty body",
			body: nil,
			err:  ErrBodyEmpty,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)
			req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			require.NoError(t, err)

			colsNames, _, values, err := adapter.ParseInsertRequest(req)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			for _, col := range tc.expectedColNames {
				require.Contains(t, colsNames, col)
			}
			expected := strings.Join(tc.expectedValues, " ")
			for _, value := range values {
				require.Contains(t, expected, value.(string))
			}
		})
	}
}

func TestParseBatchInsertRequest(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	body := []map[string]interface{}{
		{"name": "a", "age": 1},
		{"name": "b", "age": 2},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(raw))
	require.NoError(t, err)

	colsName, placeholders, values, err := adapter.ParseBatchInsertRequest(req)
	require.NoError(t, err)
	require.Contains(t, colsName, "name")
	require.Contains(t, colsName, "age")
	require.NotEmpty(t, placeholders)
	require.Len(t, values, 4)
}

func TestReturningByRequest(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodPost, "/?_returning=id&_returning=name", nil)
	require.NoError(t, err)

	ret, err := adapter.ReturningByRequest(req)
	require.NoError(t, err)
	require.Contains(t, ret, `"id"`)
	require.Contains(t, ret, `"name"`)

	req, err = http.NewRequest(http.MethodPost, "/", nil)
	require.NoError(t, err)
	ret, err = adapter.ReturningByRequest(req)
	require.NoError(t, err)
	require.Empty(t, ret)
}

func TestReturningByRequest_Branches(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodPost, "/?_returning=*", nil)
	require.NoError(t, err)
	ret, err := adapter.ReturningByRequest(req)
	require.NoError(t, err)
	require.Equal(t, "*", ret)

	req, err = http.NewRequest(http.MethodPost, "/?_returning=0bad", nil)
	require.NoError(t, err)
	_, err = adapter.ReturningByRequest(req)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidIdentifier)
}

func TestDistinctClause(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/?_distinct=true", nil)
	require.NoError(t, err)
	clause, err := adapter.DistinctClause(req)
	require.NoError(t, err)
	require.Equal(t, "SELECT DISTINCT", clause)

	req, err = http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)
	clause, err = adapter.DistinctClause(req)
	require.NoError(t, err)
	require.Empty(t, clause)
}

func TestGroupByClause(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	testCases := []struct {
		name     string
		url      string
		contains []string
		empty    bool
	}{
		{
			name:     "simple columns",
			url:      "/?_groupby=name,age",
			contains: []string{"GROUP BY", `"name"`, `"age"`},
		},
		{
			name:  "empty _groupby",
			url:   "/",
			empty: true,
		},
		{
			name:  "invalid identifier",
			url:   "/?_groupby=0name",
			empty: true,
		},
		{
			name:     "having with numeric value",
			url:      "/?_groupby=status->>having:avg:age:$gt:18",
			contains: []string{"GROUP BY", "HAVING", "AVG", "> 18"},
		},
		{
			name:     "having with string value",
			url:      "/?_groupby=status->>having:avg:age:$gt:o'brien",
			contains: []string{"GROUP BY", "HAVING", "AVG", "> 'o''brien'"},
		},
		{
			name:     "safe function expression",
			url:      "/?_groupby=upper(name)",
			contains: []string{"GROUP BY", "upper(name)"},
		},
		{
			name:     "safe function expression with regular column",
			url:      "/?_groupby=upper(name),status",
			contains: []string{"GROUP BY", "upper(name)", `"status"`},
		},
		{
			name:  "unsafe function expression with semicolon rejected",
			url:   "/?_groupby=upper(name);drop",
			empty: true,
		},
		{
			name:  "unsafe function expression with equals rejected",
			url:   "/?_groupby=upper(name=1)",
			empty: true,
		},
		{
			name:  "unbalanced parentheses in function expression rejected",
			url:   "/?_groupby=upper((name)",
			empty: true,
		},
		{
			name:  "sql line comment in function expression rejected",
			url:   "/?_groupby=upper(name)--x",
			empty: true,
		},
		{
			name:  "pg_ function expression rejected",
			url:   "/?_groupby=pg_sleep(1)",
			empty: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			clause := adapter.GroupByClause(req)
			if tc.empty {
				require.Empty(t, clause)
				return
			}
			for _, fragment := range tc.contains {
				require.Contains(t, clause, fragment)
			}
		})
	}
}

func TestGroupByClause_HavingBranches(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	tests := []struct {
		name     string
		url      string
		contains []string
		empty    bool
	}{
		{
			name:     "having with string value",
			url:      "/?_groupby=status->>having:avg:age:$gt:o'brien",
			contains: []string{"GROUP BY", "HAVING", "AVG", "> 'o''brien'"},
		},
		{
			name:     "having invalid group function falls back to group by",
			url:      "/?_groupby=status->>having:bad:age:$gt:1",
			contains: []string{"GROUP BY", `"status"`},
		},
		{
			name:     "having invalid operator falls back to group by",
			url:      "/?_groupby=status->>having:avg:age:$bad:1",
			contains: []string{"GROUP BY", `"status"`},
		},
		{
			name:     "having wrong param count falls back to group by",
			url:      "/?_groupby=status->>having:avg:age",
			contains: []string{"GROUP BY", `"status"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.url, nil)
			require.NoError(t, err)
			clause := adapter.GroupByClause(req)
			if tt.empty {
				require.Empty(t, clause)
				return
			}
			for _, fragment := range tt.contains {
				require.Contains(t, clause, fragment)
			}
		})
	}
}

func TestJoinByRequest(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/public/test?_join=inner:test2:test2.name:$eq:test.name", nil)
	require.NoError(t, err)
	joins, err := adapter.JoinByRequest(req)
	require.NoError(t, err)
	require.Len(t, joins, 1)
	require.Contains(t, joins[0], "INNER JOIN")
	require.Contains(t, joins[0], `"test2"."name" = "test"."name"`)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_join=weird:test2:test2.name:$eq:test.name", nil)
	require.NoError(t, err)
	_, err = adapter.JoinByRequest(req)
	require.Error(t, err)
}

func TestJoinByRequest_Branches(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/public/test", nil)
	require.NoError(t, err)
	joins, err := adapter.JoinByRequest(req)
	require.NoError(t, err)
	require.Nil(t, joins)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_join=left:test2:test2.name:$eq:test.name", nil)
	require.NoError(t, err)
	joins, err = adapter.JoinByRequest(req)
	require.NoError(t, err)
	require.Len(t, joins, 1)
	require.Contains(t, joins[0], "LEFT JOIN")

	req, err = http.NewRequest(http.MethodGet, "/public/test?_join=inner:t:onlyone:$eq:a.c", nil)
	require.NoError(t, err)
	_, err = adapter.JoinByRequest(req)
	require.Error(t, err)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_join=inner:t:t.c:$bad:a.c", nil)
	require.NoError(t, err)
	_, err = adapter.JoinByRequest(req)
	require.Error(t, err)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_join=inner", nil)
	require.NoError(t, err)
	_, err = adapter.JoinByRequest(req)
	require.ErrorIs(t, err, ErrJoinInvalidNumberOfArgs)
}

func TestOrderByRequest(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/public/test?_order=name,-number", nil)
	require.NoError(t, err)
	order, err := adapter.OrderByRequest(req)
	require.NoError(t, err)
	require.Contains(t, order, "ORDER BY")
	require.Contains(t, order, `"name"`)
	require.Contains(t, order, `"number" DESC`)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_order=0name", nil)
	require.NoError(t, err)
	_, err = adapter.OrderByRequest(req)
	require.Error(t, err)
}

func TestSelectFields(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	sql, err := adapter.SelectFields([]string{"name", "age"})
	require.NoError(t, err)
	require.Contains(t, sql, `"name"`)
	require.Contains(t, sql, `"age"`)

	_, err = adapter.SelectFields(nil)
	require.Error(t, err)
}

func TestSelectFields_Branches(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	sql, err := adapter.SelectFields([]string{"*", "avg:age"})
	require.NoError(t, err)
	require.Contains(t, sql, `*`)
	require.Contains(t, sql, `AVG("age")`)

	// Pre-quoted aggregates produced by the _groupby normalization path must
	// still be accepted verbatim.
	sql, err = adapter.SelectFields([]string{`SUM("salary")`})
	require.NoError(t, err)
	require.Contains(t, sql, `SUM("salary")`)

	sql, err = adapter.SelectFields([]string{`MAX("age")`})
	require.NoError(t, err)
	require.Contains(t, sql, `MAX("age")`)

	sql, err = adapter.SelectFields([]string{`AVG("age") AS "avg_age"`})
	require.NoError(t, err)
	require.Contains(t, sql, `AVG("age") AS "avg_age"`)

	// Plain identifiers are quoted.
	sql, err = adapter.SelectFields([]string{"name"})
	require.NoError(t, err)
	require.Contains(t, sql, `"name"`)

	_, err = adapter.SelectFields([]string{"0bad"})
	require.Error(t, err)
}

// TestSelectFields_Injection guards GHSA-qvx3-q8vx-9q3c: the double-quoted-substring
// fast-path let any _select field carrying a quoted alias reach raw SQL. Each payload
// must now be rejected instead of concatenated.
func TestSelectFields_Injection(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	payloads := []string{
		`(SELECT rolpassword FROM pg_authid WHERE rolname='postgres' LIMIT 1)"h"`,
		`(SELECT pg_read_file('/etc/passwd'))"f"`,
		`(SELECT version())"v"`,
		`pg_sleep(5)"s"`,
	}
	for _, p := range payloads {
		_, err := adapter.SelectFields([]string{p})
		require.ErrorIs(t, err, ErrInvalidIdentifier, "payload must be rejected: %s", p)
	}
}

// Test_sanitizeSelectField exercises the shared validation gate directly.
func Test_sanitizeSelectField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		description string
		field       string
		want        string
		wantErr     bool
	}{
		{"asterisk passes through", "*", "*", false},
		{"plain identifier is quoted", "name", `"name"`, false},
		{"dotted identifier is quoted per segment", "public.age", `"public"."age"`, false},
		{"colon-syntax aggregate is normalized", "avg:age", `AVG("age")`, false},
		{"bare aggregate keyword is a plain field", "sum", `"sum"`, false},
		{"pre-quoted aggregate is accepted", `SUM("salary")`, `SUM("salary")`, false},
		{"pre-quoted aggregate with alias is accepted", `MAX("age") AS "m"`, `MAX("age") AS "m"`, false},
		{"invalid identifier is rejected", "0bad", "", true},
		{"aliased subselect injection is rejected", `(SELECT version())"v"`, "", true},
		{"non-aggregate function is rejected", `pg_read_file('/etc/passwd')"f"`, "", true},
		{"non-whitelisted quoted func is rejected", `pg_sleep("x")`, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			got, err := sanitizeSelectField(tc.field)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrInvalidIdentifier)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCountByRequest(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/public/test?_count=true", nil)
	require.NoError(t, err)
	count, err := adapter.CountByRequest(req)
	require.NoError(t, err)
	require.Contains(t, count, "COUNT")
}

func TestCountByRequest_WithSelect(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/public/test?_count=name&_select=age", nil)
	require.NoError(t, err)
	count, err := adapter.CountByRequest(req)
	require.NoError(t, err)
	require.Contains(t, count, `COUNT("name")`)
	// _select value is now validated and quoted, not concatenated raw.
	require.Contains(t, count, `, "age"`)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_count=0bad", nil)
	require.NoError(t, err)
	_, err = adapter.CountByRequest(req)
	require.Error(t, err)
}

// TestCountByRequest_Injection guards the second sink: _select interpolated into
// SELECT COUNT(...)%s FROM. Crafted projections from GHSA-qvx3-q8vx-9q3c must be
// rejected, not executed.
func TestCountByRequest_Injection(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	payloads := []string{
		`(SELECT rolpassword FROM pg_authid WHERE rolname='postgres' LIMIT 1)"h"`,
		`(SELECT pg_read_file('/etc/passwd'))"f"`,
		`(SELECT version())"v"`,
		`pg_sleep(5)"s"`,
		`celphone,(SELECT 1)"x"`,
		`pg_sleep("x")`,
	}
	for _, p := range payloads {
		req, err := http.NewRequest(http.MethodGet,
			"/public/test?_count=*&_select="+url.QueryEscape(p), nil)
		require.NoError(t, err)
		_, err = adapter.CountByRequest(req)
		require.ErrorIs(t, err, ErrInvalidIdentifier, "payload must be rejected: %s", p)
	}
}

func TestPaginateIfPossible(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	testCases := []struct {
		name        string
		url         string
		expectEmpty bool
		expectErr   bool
		contains    []string
	}{
		{
			name:     "valid page and size",
			url:      "/public/test?_page=1&_page_size=10",
			contains: []string{"LIMIT", "OFFSET"},
		},
		{
			name:        "missing page param",
			url:         "/public/test?_page_size=10",
			expectEmpty: true,
		},
		{
			name:      "invalid page number",
			url:       "/public/test?_page=abc",
			expectErr: true,
		},
		{
			name:      "invalid page size",
			url:       "/public/test?_page=1&_page_size=abc",
			expectErr: true,
		},
		{
			name:     "default page size",
			url:      "/public/test?_page=2",
			contains: []string{"LIMIT 10", "OFFSET"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)

			page, err := adapter.PaginateIfPossible(req)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.expectEmpty {
				require.Empty(t, page)
				return
			}
			for _, fragment := range tc.contains {
				require.Contains(t, page, fragment)
			}
		})
	}
}

func TestCatalogSQLBuilders(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	testCases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "DatabaseWhere empty",
			got:  adapter.DatabaseWhere(""),
			want: statements.DatabasesWhere,
		},
		{
			name: "DatabaseWhere with request",
			got:  adapter.DatabaseWhere("extra = 1"),
			want: fmt.Sprintf("%v AND extra = 1", statements.DatabasesWhere),
		},
		{
			name: "DatabaseOrderBy explicit",
			got:  adapter.DatabaseOrderBy("ORDER BY x", true),
			want: "ORDER BY x",
		},
		{
			name: "DatabaseOrderBy empty with count",
			got:  adapter.DatabaseOrderBy("", true),
			want: "",
		},
		{
			name: "DatabaseOrderBy empty without count",
			got:  adapter.DatabaseOrderBy("", false),
			want: fmt.Sprintf("\nORDER BY\n\t%s ASC", statements.FieldDatabaseName),
		},
		{
			name: "SchemaOrderBy explicit",
			got:  adapter.SchemaOrderBy("ORDER BY y", true),
			want: "ORDER BY y",
		},
		{
			name: "SchemaOrderBy empty with count",
			got:  adapter.SchemaOrderBy("", true),
			want: "",
		},
		{
			name: "SchemaOrderBy empty without count",
			got:  adapter.SchemaOrderBy("", false),
			want: fmt.Sprintf("\nORDER BY\n\t%s ASC", statements.FieldSchemaName),
		},
		{
			name: "TableClause",
			got:  adapter.TableClause(),
			want: statements.TablesSelect,
		},
		{
			name: "TableWhere empty",
			got:  adapter.TableWhere(""),
			want: statements.TablesWhere,
		},
		{
			name: "TableWhere with request",
			got:  adapter.TableWhere("requestWhere"),
			want: fmt.Sprintf("%v AND requestWhere", statements.TablesWhere),
		},
		{
			name: "TableOrderBy explicit",
			got:  adapter.TableOrderBy("ORDER BY z"),
			want: "ORDER BY z",
		},
		{
			name: "TableOrderBy empty",
			got:  adapter.TableOrderBy(""),
			want: statements.TablesOrderBy,
		},
		{
			name: "SchemaTablesClause",
			got:  adapter.SchemaTablesClause(),
			want: statements.SchemaTablesSelect,
		},
		{
			name: "SchemaTablesWhere empty",
			got:  adapter.SchemaTablesWhere(""),
			want: statements.SchemaTablesWhere,
		},
		{
			name: "SchemaTablesWhere with request",
			got:  adapter.SchemaTablesWhere("requestWhere"),
			want: fmt.Sprintf("%v AND requestWhere", statements.SchemaTablesWhere),
		},
		{
			name: "SchemaTablesOrderBy explicit",
			got:  adapter.SchemaTablesOrderBy("ORDER BY t"),
			want: "ORDER BY t",
		},
		{
			name: "SchemaTablesOrderBy empty",
			got:  adapter.SchemaTablesOrderBy(""),
			want: statements.SchemaTablesOrderBy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
		})
	}
}

func TestNormalizeColumn(t *testing.T) {
	t.Parallel()

	got, err := normalizeColumn("name")
	require.NoError(t, err)
	require.Equal(t, "name", got)

	got, err = normalizeColumn("avg:age")
	require.NoError(t, err)
	require.Equal(t, `AVG("age")`, got)

	_, err = normalizeColumn("invalid:age")
	require.Error(t, err)
}

func TestNormalizeAll(t *testing.T) {
	t.Parallel()

	cols, err := normalizeAll([]string{"name", "max:age"})
	require.NoError(t, err)
	require.Equal(t, []string{"name", `MAX("age")`}, cols)

	_, err = normalizeAll([]string{"bad:col"})
	require.Error(t, err)
}

func TestDatabaseClause(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/databases", nil)
	require.NoError(t, err)
	query, hasCount := adapter.DatabaseClause(req)
	require.False(t, hasCount)
	require.Contains(t, query, "datname")

	req, err = http.NewRequest(http.MethodGet, "/databases?_count=true", nil)
	require.NoError(t, err)
	_, hasCount = adapter.DatabaseClause(req)
	require.True(t, hasCount)
}

func TestSchemaClause(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/schemas", nil)
	require.NoError(t, err)
	query, hasCount := adapter.SchemaClause(req)
	require.False(t, hasCount)
	require.Contains(t, query, "schema_name")

	req, err = http.NewRequest(http.MethodGet, "/schemas?_count=true", nil)
	require.NoError(t, err)
	_, hasCount = adapter.SchemaClause(req)
	require.True(t, hasCount)
}

func TestSelectInsertDeleteUpdateSQL(t *testing.T) {

	t.Parallel()

	adapter := testAdapter()

	selectSQL := adapter.SelectSQL("SELECT", "db", "public", "users")
	require.Contains(t, selectSQL, `"db"."public"."users"`)

	insertSQL := adapter.InsertSQL("db", "public", "users", "name", "$1")
	require.Contains(t, insertSQL, "INSERT INTO")
	require.Contains(t, insertSQL, "users")

	deleteSQL := adapter.DeleteSQL("db", "public", "users")
	require.Contains(t, deleteSQL, "DELETE FROM")

	updateSQL := adapter.UpdateSQL("db", "public", "users", `"name"=$1`)
	require.Contains(t, updateSQL, "UPDATE")
	require.Contains(t, updateSQL, `"name"=$1`)
}

func TestPhysicalNameWithRegistry(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		PGDatabase: "legacy-db",
		Databases: []config.DatabaseConf{
			{Alias: "tenant-a", Database: "app_a"},
			{Alias: "tenant-b", Database: "app_b", URL: "postgres://u:p@host:5432/app_b"},
		},
	})

	require.Equal(t, "app_a", adapter.PhysicalName("tenant-a"))
	require.Equal(t, "app_b", adapter.PhysicalName("tenant-b"))
	require.Equal(t, "legacy-db", adapter.PhysicalName(""))
	require.Equal(t, "legacy-db", adapter.PhysicalName("legacy-db"))
}

func TestSelectSQLUsesPhysicalName(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		Databases: []config.DatabaseConf{
			{Alias: "tenant-a", Database: "app_a"},
		},
	})
	sql := adapter.SelectSQL("SELECT", "tenant-a", "public", "users")
	require.Contains(t, sql, `"public"."users"`)
	require.NotContains(t, sql, `"app_a"`)
}

func TestIsRegistered(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		Databases: []config.DatabaseConf{{Alias: "tenant-a", Database: "app_a"}},
	})
	require.True(t, adapter.IsRegistered("tenant-a"))
	require.False(t, adapter.IsRegistered("unknown"))

	legacyAdapter := testAdapter(defaultTestConf())
	require.True(t, legacyAdapter.IsRegistered("anything"))
}

func TestTablePermissionsTenantPrecedence(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		AccessConf: config.AccessConf{
			Restrict: true,
			Tables: []config.TablesConf{
				{Name: "users", Permissions: []string{"read"}, Fields: []string{"id"}},
				{Schema: "public", Name: "users", Permissions: []string{"write"}, Fields: []string{"name"}},
				{Database: "tenant-a", Schema: "public", Name: "users", Permissions: []string{"delete"}, Fields: []string{"email"}},
			},
		},
	})

	require.True(t, adapter.TablePermissions("tenant-a", "public", "users", "delete", ""))
	require.False(t, adapter.TablePermissions("tenant-a", "public", "users", "read", ""))
	require.True(t, adapter.TablePermissions("tenant-b", "public", "users", "write", ""))
	require.True(t, adapter.TablePermissions("tenant-b", "other", "users", "read", ""))
}

func TestTablePermissions(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(permissionTestConf())

	testCases := []struct {
		name       string
		table      string
		permission string
		userName   string
		want       bool
	}{
		{"read allowed", "test_readonly_access", "read", "", true},
		{"read denied", "test_write_and_delete_access", "read", "", false},
		{"write allowed", "test_write_and_delete_access", "write", "", true},
		{"write denied", "test_readonly_access", "write", "", false},
		{"ignored table", "ignored_table", "write", "", true},
		{"missing table", "test_permission_does_not_exist", "read", "", false},
		{"user write override", "no_user_write_table", "write", "foo_read", true},
		{"user read denied on write-only", "no_user_write_table", "read", "foo_read", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := adapter.TablePermissions("", "", tc.table, tc.permission, tc.userName)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestTablePermissionsUnrestrict(t *testing.T) {
	t.Parallel()

	cfg := permissionTestConf()
	cfg.AccessConf.Restrict = false
	adapter := testAdapter(cfg)

	got := adapter.TablePermissions("", "", "any_table", "read", "")
	require.True(t, got)
}

func TestFieldsPermissions(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(permissionTestConf())

	req, err := http.NewRequest(http.MethodGet, "/public/test?_select=name,surname", nil)
	require.NoError(t, err)
	fields, err := adapter.FieldsPermissions(req, "", "public", "test_fields_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name", "surname"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "", "public", "test_fields_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name", "surname"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_select=name,surname", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "", "public", "test_fields_access", "delete", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name", "surname"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_select=name", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "", "public", "test_readonly_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "", "public", "test_readonly_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"*"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_select=max:age&_groupby=status", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "", "public", "test_readonly_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{`MAX("age")`}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "", "public", "no_user_write_table", "write", "foo_read")
	require.NoError(t, err)
	require.Equal(t, []string{"name"}, fields)
}

func TestFieldsPermissions_RestrictBranches(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(permissionTestConf())

	req, err := http.NewRequest(http.MethodGet, "/public/test?_select=name", nil)
	require.NoError(t, err)
	fields, err := adapter.FieldsPermissions(req, "", "public", "test_readonly_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_select=invalid:field&_groupby=status", nil)
	require.NoError(t, err)
	_, err = adapter.FieldsPermissions(req, "", "public", "test_readonly_access", "read", "")
	require.Error(t, err)
}

func TestFieldsByPermission(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(permissionTestConf())

	fields := adapter.fieldsByPermission("", "public", "test_fields_access", "read", "")
	require.Equal(t, []string{"name", "surname"}, fields)

	fields = adapter.fieldsByPermission("", "public", "test_write_and_delete_access", "read", "foo_read")
	require.Equal(t, []string{"*"}, fields)

	fields = adapter.fieldsByPermission("", "public", "no_user_write_table", "write", "foo_read")
	require.Equal(t, []string{"name"}, fields)
}

func TestCheckField(t *testing.T) {
	t.Parallel()

	fields := []string{"name", "age"}
	require.Equal(t, "name", checkField("name", fields))
	require.Equal(t, `SUM("age")`, checkField(`SUM("age")`, fields))
	require.Empty(t, checkField("missing", fields))
}

func Test_isTopLevelOrSeparator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    string
		i    int
		want bool
	}{
		{
			name: "top level OR with spaces",
			v:    "name=$eq.a OR phone=$eq.b",
			i:    11,
			want: true,
		},
		{
			name: "case insensitive or",
			v:    "name=$eq.a or phone=$eq.b",
			i:    11,
			want: true,
		},
		{
			name: "OR at start with trailing space",
			v:    "OR name=$eq.a",
			i:    0,
			want: true,
		},
		{
			name: "legacy pipe separator not OR",
			v:    "name=$eq.a||phone=$eq.b",
			i:    11,
			want: false,
		},
		{
			name: "OR inside word",
			v:    "color=$eq.red",
			i:    1,
			want: false,
		},
		{
			name: "OR without leading whitespace",
			v:    "aOR b",
			i:    1,
			want: false,
		},
		{
			name: "OR without trailing whitespace",
			v:    "a ORb",
			i:    2,
			want: false,
		},
		{
			name: "OR at end of string",
			v:    "a OR",
			i:    2,
			want: false,
		},
		{
			name: "index past string",
			v:    "a OR b",
			i:    10,
			want: false,
		},
		{
			name: "single character string",
			v:    "O",
			i:    0,
			want: false,
		},
		{
			name: "index on space before OR",
			v:    `title=$eq."foo OR bar"`,
			i:    14,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTopLevelOrSeparator(tt.v, tt.i)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_splitTopLevelOrGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    string
		want []string
	}{
		{
			name: "explicit OR separator",
			v:    "name=$eq.prest OR phoneNumber=$eq.123",
			want: []string{"name=$eq.prest", "phoneNumber=$eq.123"},
		},
		{
			name: "case insensitive or",
			v:    "name=$ilike.%25val%25 or phoneNumber=$eq.123",
			want: []string{"name=$ilike.%25val%25", "phoneNumber=$eq.123"},
		},
		{
			name: "legacy pipe separator",
			v:    "name=$eq.prest||phoneNumber=$eq.123",
			want: []string{"name=$eq.prest", "phoneNumber=$eq.123"},
		},
		{
			name: "three parts",
			v:    "a=$eq.1 OR b=$eq.2 OR c=$eq.3",
			want: []string{"a=$eq.1", "b=$eq.2", "c=$eq.3"},
		},
		{
			name: "single part without separator",
			v:    "name=$eq.prest",
			want: []string{"name=$eq.prest"},
		},
		{
			name: "OR inside double quoted value",
			v:    `title=$eq."foo OR bar" OR phoneNumber=$eq.123`,
			want: []string{`title=$eq."foo OR bar"`, "phoneNumber=$eq.123"},
		},
		{
			name: "OR inside single quoted value",
			v:    "title=$eq.'foo OR bar' OR phoneNumber=$eq.123",
			want: []string{"title=$eq.'foo OR bar'", "phoneNumber=$eq.123"},
		},
		{
			name: "escaped double quote inside value",
			v:    `name=$eq."foo""bar" OR age=$gt.18`,
			want: []string{`name=$eq."foo""bar"`, "age=$gt.18"},
		},
		{
			name: "OR inside word not split",
			v:    "color=$eq.red",
			want: []string{"color=$eq.red"},
		},
		{
			name: "empty string",
			v:    "",
			want: []string{},
		},
		{
			name: "trims surrounding whitespace",
			v:    "  name=$eq.a   OR   phone=$eq.b  ",
			want: []string{"name=$eq.a", "phone=$eq.b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitTopLevelOrGroup(tt.v)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSplitTopLevelOrGroup_EscapedSingleQuote(t *testing.T) {
	t.Parallel()

	got := splitTopLevelOrGroup("name=$eq.'foo''bar' OR age=$gt.18")
	require.Equal(t, []string{"name=$eq.'foo''bar'", "age=$gt.18"}, got)
}

func Test_postgres_whereKeyAndValue(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	tests := []struct {
		name     string
		rawKey   string
		v        string
		wantKey  string
		wantVals []interface{}
		wantErr  error
		errMsg   string
	}{
		{
			name:     "explicit equality",
			rawKey:   "name",
			v:        "$eq.prest",
			wantKey:  `"name" = $1`,
			wantVals: []interface{}{"prest"},
		},
		{
			name:     "implicit equality",
			rawKey:   "name",
			v:        "prest",
			wantKey:  `"name" = $1`,
			wantVals: []interface{}{"prest"},
		},
		{
			name:     "table alias",
			rawKey:   "c.test",
			v:        "$eq.cool",
			wantKey:  `"c"."test" = $1`,
			wantVals: []interface{}{"cool"},
		},
		{
			name:     "like operator",
			rawKey:   "name",
			v:        "$like.%25val%25",
			wantKey:  `"name" LIKE $1`,
			wantVals: []interface{}{"%25val%25"},
		},
		{
			name:     "in operator",
			rawKey:   "name",
			v:        "$in.foo,bar",
			wantKey:  `"name" IN ($1,$2)`,
			wantVals: []interface{}{"foo", "bar"},
		},
		{
			name:     "not in operator",
			rawKey:   "name",
			v:        "$nin.foo,bar",
			wantKey:  `"name" NOT IN ($1,$2)`,
			wantVals: []interface{}{"foo", "bar"},
		},
		{
			name:     "any operator",
			rawKey:   "name",
			v:        "$any.foo,bar",
			wantKey:  `"name" = ANY ($1)`,
			wantVals: []interface{}{`{"foo","bar"}`},
		},
		{
			name:     "some operator",
			rawKey:   "name",
			v:        "$some.foo,bar",
			wantKey:  `"name" = SOME ($1)`,
			wantVals: []interface{}{`{"foo","bar"}`},
		},
		{
			name:     "all operator",
			rawKey:   "name",
			v:        "$all.foo,bar",
			wantKey:  `"name" = ALL ($1)`,
			wantVals: []interface{}{`{"foo","bar"}`},
		},
		{
			name:    "is null operator",
			rawKey:  "name",
			v:       "$null.",
			wantKey: `"name" IS NULL`,
		},
		{
			name:    "is not null operator",
			rawKey:  "name",
			v:       "$notnull.",
			wantKey: `"name" IS NOT NULL`,
		},
		{
			name:    "is true operator",
			rawKey:  "name",
			v:       "$true.",
			wantKey: `"name" IS TRUE`,
		},
		{
			name:    "is not true operator",
			rawKey:  "name",
			v:       "$nottrue.",
			wantKey: `"name" IS NOT TRUE`,
		},
		{
			name:    "is false operator",
			rawKey:  "name",
			v:       "$false.",
			wantKey: `"name" IS FALSE`,
		},
		{
			name:    "is not false operator",
			rawKey:  "name",
			v:       "$notfalse.",
			wantKey: `"name" IS NOT FALSE`,
		},
		{
			name:     "jsonb not in operator",
			rawKey:   "data->>status:jsonb",
			v:        "$nin.open,closed",
			wantKey:  `"data"->>'status' NOT IN ($1,$2)`,
			wantVals: []interface{}{"open", "closed"},
		},
		{
			name:     "jsonb any operator",
			rawKey:   "data->>status:jsonb",
			v:        "$any.open,closed",
			wantKey:  `"data"->>'status' = ANY ($1)`,
			wantVals: []interface{}{`{"open","closed"}`},
		},
		{
			name:    "jsonb is not null operator",
			rawKey:  "data->>status:jsonb",
			v:       "$notnull.",
			wantKey: `"data"->>'status' IS NOT NULL`,
		},
		{
			name:     "jsonb field",
			rawKey:   "data->>description:jsonb",
			v:        "$eq.testing",
			wantKey:  `"data"->>'description' = $1`,
			wantVals: []interface{}{"testing"},
		},
		{
			name:    "tsquery field",
			rawKey:  "name:tsquery",
			v:       "prest",
			wantKey: `name @@ to_tsquery('prest')`,
		},
		{
			name:     "ltree left ancestor",
			rawKey:   "path",
			v:        "$ltreelanc.Top.*",
			wantKey:  `"path" @> $1`,
			wantVals: []interface{}{"Top.*"},
		},
		{
			name:    "empty value",
			rawKey:  "name",
			v:       "",
			wantErr: ErrInvalidOperator,
		},
		{
			name:    "invalid operator",
			rawKey:  "name",
			v:       "$bad.value",
			wantErr: ErrInvalidOperator,
		},
		{
			name:    "invalid identifier",
			rawKey:  "0name",
			v:       "$eq.x",
			wantErr: ErrInvalidIdentifier,
		},
		{
			name:   "unknown type suffix",
			rawKey: "name:unknown",
			v:      "$eq.x",
			errMsg: "unknown type suffix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pid := 1
			gotKey, gotVals, err := adapter.whereKeyAndValue(tt.rawKey, tt.v, &pid)

			if tt.errMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				return
			}

			if tt.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantKey, gotKey)
			require.Equal(t, tt.wantVals, gotVals)
		})
	}
}

func Test_sliceToJSONList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ifaceSlice interface{}
		want       string
		wantErr    error
	}{
		{
			name:       "nil slice",
			ifaceSlice: nil,
			want:       "[]",
			wantErr:    ErrEmptyOrInvalidSlice,
		},
		{
			name:       "empty string slice",
			ifaceSlice: []string{},
			want:       "[]",
		},
		{
			name:       "string values quoted",
			ifaceSlice: []string{"a", "b"},
			want:       `["a", "b"]`,
		},
		{
			name:       "int values unquoted",
			ifaceSlice: []int{1, 2, 3},
			want:       `[1, 2, 3]`,
		},
		{
			name:       "float64 values unquoted",
			ifaceSlice: []float64{1.5, 2},
			want:       `[1.5, 2]`,
		},
		{
			name:       "mixed interface slice",
			ifaceSlice: []interface{}{1, "two", 3.5},
			want:       `[1, "two", 3.5]`,
		},
		{
			name:       "single string element",
			ifaceSlice: []string{"only"},
			want:       `["only"]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sliceToJSONList(tt.ifaceSlice)

			if tt.wantErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.wantErr)
				require.Equal(t, tt.want, got)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_postgres_BatchInsertCopy(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) *postgres
		dbname  string
		schema  string
		table   string
		keys    []string
		values  []interface{}
		wantErr string
	}{
		{
			name: "connection error",
			setup: func(t *testing.T) *postgres {
				return withFailingDBConnect(t, "connect failed")
			},
			dbname:  defaultMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{"name"},
			values:  []interface{}{"alice"},
			wantErr: "connect",
		},
		{
			name: "begin error",
			setup: func(t *testing.T) *postgres {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
				return adapter
			},
			dbname:  defaultMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{"name"},
			values:  []interface{}{"alice"},
			wantErr: "begin failed",
		},
		{
			name: "invalid quoted key",
			setup: func(t *testing.T) *postgres {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				mock.ExpectRollback()
				return adapter
			},
			dbname:  defaultMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{`"unclosed`},
			values:  []interface{}{"alice"},
			wantErr: "invalid syntax",
		},
		{
			name: "prepare error",
			setup: func(t *testing.T) *postgres {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				mock.ExpectPrepare(`COPY "public"."users"`).WillReturnError(errors.New("prepare failed"))
				mock.ExpectRollback()
				return adapter
			},
			dbname:  defaultMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{"name"},
			values:  []interface{}{"alice"},
			wantErr: "prepare failed",
		},
		{
			name: "exec error",
			setup: func(t *testing.T) *postgres {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				prep := mock.ExpectPrepare(`COPY "public"."users"`)
				prep.ExpectExec().WithArgs("alice").WillReturnError(errors.New("exec failed"))
				mock.ExpectRollback()
				return adapter
			},
			dbname:  defaultMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{"name"},
			values:  []interface{}{"alice"},
			wantErr: "exec failed",
		},
		{
			name: "success single row",
			setup: func(t *testing.T) *postgres {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				prep := mock.ExpectPrepare(`COPY "public"."users"`)
				prep.ExpectExec().WithArgs("alice").WillReturnResult(sqlmock.NewResult(0, 1))
				prep.ExpectExec().WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectCommit()
				return adapter
			},
			dbname: defaultMockDB,
			schema: "public",
			table:  "users",
			keys:   []string{"name"},
			values: []interface{}{"alice"},
		},
		{
			name: "success quoted keys",
			setup: func(t *testing.T) *postgres {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				prep := mock.ExpectPrepare(`COPY "public"."users"`)
				prep.ExpectExec().WithArgs("a", 1).WillReturnResult(sqlmock.NewResult(0, 1))
				prep.ExpectExec().WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectCommit()
				return adapter
			},
			dbname: defaultMockDB,
			schema: "public",
			table:  "users",
			keys:   []string{`"name"`, `"age"`},
			values: []interface{}{"a", 1},
		},
		{
			name: "success two rows",
			setup: func(t *testing.T) *postgres {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				prep := mock.ExpectPrepare(`COPY "public"."users"`)
				prep.ExpectExec().WithArgs("a", 1).WillReturnResult(sqlmock.NewResult(0, 1))
				prep.ExpectExec().WithArgs("b", 2).WillReturnResult(sqlmock.NewResult(0, 1))
				prep.ExpectExec().WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectCommit()
				return adapter
			},
			dbname: defaultMockDB,
			schema: "public",
			table:  "users",
			keys:   []string{"name", "age"},
			values: []interface{}{"a", 1, "b", 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := New(defaultTestConf()).(*postgres)
			if tt.setup != nil {
				adapter = tt.setup(t)
			}

			sc := adapter.BatchInsertCopy(tt.dbname, tt.schema, tt.table, tt.keys, tt.values...)
			if tt.wantErr != "" {
				require.Error(t, sc.Err())
				require.Contains(t, sc.Err().Error(), tt.wantErr)
				return
			}
			require.NoError(t, sc.Err())
		})
	}
}

func Test_postgres_BatchInsertCopyCtx(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (*postgres, context.Context)
		dbname  string
		schema  string
		table   string
		keys    []string
		values  []interface{}
		wantErr string
	}{
		{
			name: "connection error for context database",
			setup: func(t *testing.T) (*postgres, context.Context) {
				adapter := withFailingDBConnect(t, "connect failed")
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				return adapter, ctx
			},
			dbname:  contextMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{"name"},
			values:  []interface{}{"alice"},
			wantErr: "connect",
		},
		{
			name: "begin error",
			setup: func(t *testing.T) (*postgres, context.Context) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectBegin().WillReturnError(errors.New("begin failed"))
				return adapter, ctx
			},
			dbname:  contextMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{"name"},
			values:  []interface{}{"alice"},
			wantErr: "begin failed",
		},
		{
			name: "invalid quoted key",
			setup: func(t *testing.T) (*postgres, context.Context) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectBegin()
				ctxMock.ExpectRollback()
				return adapter, ctx
			},
			dbname:  contextMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{`"unclosed`},
			values:  []interface{}{"alice"},
			wantErr: "invalid syntax",
		},
		{
			name: "prepare error",
			setup: func(t *testing.T) (*postgres, context.Context) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectBegin()
				ctxMock.ExpectPrepare(`COPY "public"."users"`).WillReturnError(errors.New("prepare failed"))
				ctxMock.ExpectRollback()
				return adapter, ctx
			},
			dbname:  contextMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{"name"},
			values:  []interface{}{"alice"},
			wantErr: "prepare failed",
		},
		{
			name: "exec error",
			setup: func(t *testing.T) (*postgres, context.Context) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectBegin()
				prep := ctxMock.ExpectPrepare(`COPY "public"."users"`)
				prep.ExpectExec().WithArgs("alice").WillReturnError(errors.New("exec failed"))
				ctxMock.ExpectRollback()
				return adapter, ctx
			},
			dbname:  contextMockDB,
			schema:  "public",
			table:   "users",
			keys:    []string{"name"},
			values:  []interface{}{"alice"},
			wantErr: "exec failed",
		},
		{
			name: "success single row",
			setup: func(t *testing.T) (*postgres, context.Context) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectBegin()
				prep := ctxMock.ExpectPrepare(`COPY "public"."users"`)
				prep.ExpectExec().WithArgs("alice").WillReturnResult(sqlmock.NewResult(0, 1))
				prep.ExpectExec().WillReturnResult(sqlmock.NewResult(0, 0))
				ctxMock.ExpectCommit()
				return adapter, ctx
			},
			dbname: contextMockDB,
			schema: "public",
			table:  "users",
			keys:   []string{"name"},
			values: []interface{}{"alice"},
		},
		{
			name: "success quoted keys",
			setup: func(t *testing.T) (*postgres, context.Context) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectBegin()
				prep := ctxMock.ExpectPrepare(`COPY "public"."users"`)
				prep.ExpectExec().WithArgs("a", 1).WillReturnResult(sqlmock.NewResult(0, 1))
				prep.ExpectExec().WillReturnResult(sqlmock.NewResult(0, 0))
				ctxMock.ExpectCommit()
				return adapter, ctx
			},
			dbname: contextMockDB,
			schema: "public",
			table:  "users",
			keys:   []string{`"name"`, `"age"`},
			values: []interface{}{"a", 1},
		},
		{
			name: "success two rows",
			setup: func(t *testing.T) (*postgres, context.Context) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectBegin()
				prep := ctxMock.ExpectPrepare(`COPY "public"."users"`)
				prep.ExpectExec().WithArgs("a", 1).WillReturnResult(sqlmock.NewResult(0, 1))
				prep.ExpectExec().WithArgs("b", 2).WillReturnResult(sqlmock.NewResult(0, 1))
				prep.ExpectExec().WillReturnResult(sqlmock.NewResult(0, 0))
				ctxMock.ExpectCommit()
				return adapter, ctx
			},
			dbname: contextMockDB,
			schema: "public",
			table:  "users",
			keys:   []string{"name", "age"},
			values: []interface{}{"a", 1, "b", 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, ctx := New(defaultTestConf()).(*postgres), context.Background()
			if tt.setup != nil {
				adapter, ctx = tt.setup(t)
			}

			sc := adapter.BatchInsertCopyCtx(ctx, tt.dbname, tt.schema, tt.table, tt.keys, tt.values...)
			if tt.wantErr != "" {
				require.Error(t, sc.Err())
				require.Contains(t, sc.Err().Error(), tt.wantErr)
				return
			}
			require.NoError(t, sc.Err())
		})
	}
}

func Test_postgres_delete(t *testing.T) {
	t.Parallel()

	const deleteSQL = `DELETE FROM "test"."public"."users" WHERE "id"=$1`

	tests := []struct {
		name     string
		setup    func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx)
		sql      string
		params   []interface{}
		wantJSON string
		wantErr  string
	}{
		{
			name: "prepare error via db",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`DELETE FROM`).WillReturnError(errors.New("prepare failed"))
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:     deleteSQL,
			params:  []interface{}{1},
			wantErr: "prepare failed",
		},
		{
			name: "exec error via db",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`DELETE FROM`).
					ExpectExec().
					WithArgs(1).
					WillReturnError(errors.New("exec failed"))
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:     deleteSQL,
			params:  []interface{}{1},
			wantErr: "exec failed",
		},
		{
			name: "success rows affected",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`DELETE FROM`).
					ExpectExec().
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:      deleteSQL,
			params:   []interface{}{1},
			wantJSON: `{"rows_affected":1}`,
		},
		{
			name: "success with context",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectPrepare(`DELETE FROM`).
					ExpectExec().
					WithArgs(2).
					WillReturnResult(sqlmock.NewResult(0, 2))
				db, err := adapter.conn.GetFromPool(contextMockDB)
				require.NoError(t, err)
				return adapter, ctx, db, nil
			},
			sql:      deleteSQL,
			params:   []interface{}{2},
			wantJSON: `{"rows_affected":2}`,
		},
		{
			name: "prepare error via transaction",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				mock.ExpectPrepare(`DELETE FROM`).WillReturnError(errors.New("prepare failed"))
				tx, err := adapter.GetTransaction()
				require.NoError(t, err)
				return adapter, nil, nil, tx
			},
			sql:     deleteSQL,
			params:  []interface{}{1},
			wantErr: "prepare failed",
		},
		{
			name: "success via transaction",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				mock.ExpectPrepare(`DELETE FROM`).
					ExpectExec().
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
				tx, err := adapter.GetTransaction()
				require.NoError(t, err)
				return adapter, nil, nil, tx
			},
			sql:      deleteSQL,
			params:   []interface{}{1},
			wantJSON: `{"rows_affected":1}`,
		},
		{
			name: "returning rows",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`DELETE FROM`).
					ExpectQuery().
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "alice"))
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:      `DELETE FROM "test"."public"."users" WHERE "id"=$1 RETURNING "id","name"`,
			params:   []interface{}{1},
			wantJSON: `[{"id":1,"name":"alice"}]`,
		},
		{
			name: "rows affected error",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`DELETE FROM`).
					ExpectExec().
					WithArgs(1).
					WillReturnResult(errRowsAffectedResult{})
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:     deleteSQL,
			params:  []interface{}{1},
			wantErr: "rows affected failed",
		},
		{
			name: "success with context and transaction",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				mock.ExpectBegin()
				mock.ExpectPrepare(`DELETE FROM`).
					ExpectExec().
					WithArgs(3).
					WillReturnResult(sqlmock.NewResult(0, 1))
				tx, err := adapter.GetTransaction()
				require.NoError(t, err)
				return adapter, ctx, nil, tx
			},
			sql:      deleteSQL,
			params:   []interface{}{3},
			wantJSON: `{"rows_affected":1}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, ctx, db, tx := New(defaultTestConf()).(*postgres), context.Background(), (*sqlx.DB)(nil), (*sql.Tx)(nil)
			if tt.setup != nil {
				adapter, ctx, db, tx = tt.setup(t)
			}

			sc := adapter.delete(ctx, db, tx, tt.sql, tt.params...)
			if tt.wantErr != "" {
				require.Error(t, sc.Err())
				require.Contains(t, sc.Err().Error(), tt.wantErr)
				return
			}
			require.NoError(t, sc.Err())
			require.JSONEq(t, tt.wantJSON, string(sc.Bytes()))
		})
	}
}

func Test_postgres_update(t *testing.T) {
	t.Parallel()

	const updateSQL = `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`

	tests := []struct {
		name     string
		setup    func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx)
		sql      string
		params   []interface{}
		wantJSON string
		wantErr  string
	}{
		{
			name: "prepare error via db",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`UPDATE "test"."public"."users"`).WillReturnError(errors.New("prepare failed"))
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:     updateSQL,
			params:  []interface{}{"bob", 1},
			wantErr: "prepare failed",
		},
		{
			name: "exec error via db",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
					ExpectExec().
					WithArgs("bob", 1).
					WillReturnError(errors.New("exec failed"))
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:     updateSQL,
			params:  []interface{}{"bob", 1},
			wantErr: "exec failed",
		},
		{
			name: "success rows affected",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
					ExpectExec().
					WithArgs("bob", 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:      updateSQL,
			params:   []interface{}{"bob", 1},
			wantJSON: `{"rows_affected":1}`,
		},
		{
			name: "success with context",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, _, ctxMock := withSQLMocks(t)
				ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
				ctxMock.ExpectPrepare(`UPDATE "test"."public"."users"`).
					ExpectExec().
					WithArgs("carol", 2).
					WillReturnResult(sqlmock.NewResult(0, 2))
				db, err := adapter.conn.GetFromPool(contextMockDB)
				require.NoError(t, err)
				return adapter, ctx, db, nil
			},
			sql:      updateSQL,
			params:   []interface{}{"carol", 2},
			wantJSON: `{"rows_affected":2}`,
		},
		{
			name: "prepare error via transaction",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				mock.ExpectPrepare(`UPDATE "test"."public"."users"`).WillReturnError(errors.New("prepare failed"))
				tx, err := adapter.GetTransaction()
				require.NoError(t, err)
				return adapter, nil, nil, tx
			},
			sql:     updateSQL,
			params:  []interface{}{"bob", 1},
			wantErr: "prepare failed",
		},
		{
			name: "success via transaction",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectBegin()
				mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
					ExpectExec().
					WithArgs("bob", 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
				tx, err := adapter.GetTransaction()
				require.NoError(t, err)
				return adapter, nil, nil, tx
			},
			sql:      updateSQL,
			params:   []interface{}{"bob", 1},
			wantJSON: `{"rows_affected":1}`,
		},
		{
			name: "returning rows",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
					ExpectQuery().
					WithArgs("bob", 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(1), "bob"))
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:      `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2 RETURNING "id","name"`,
			params:   []interface{}{"bob", 1},
			wantJSON: `[{"id":1,"name":"bob"}]`,
		},
		{
			name: "rows affected error",
			setup: func(t *testing.T) (*postgres, context.Context, *sqlx.DB, *sql.Tx) {
				adapter, mock := withSQLMock(t)
				mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
					ExpectExec().
					WithArgs("bob", 1).
					WillReturnResult(errRowsAffectedResult{})
				db, err := adapter.DB()
				require.NoError(t, err)
				return adapter, nil, db, nil
			},
			sql:     updateSQL,
			params:  []interface{}{"bob", 1},
			wantErr: "rows affected failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, ctx, db, tx := New(defaultTestConf()).(*postgres), context.Background(), (*sqlx.DB)(nil), (*sql.Tx)(nil)
			if tt.setup != nil {
				adapter, ctx, db, tx = tt.setup(t)
			}

			sc := adapter.update(ctx, db, tx, tt.sql, tt.params...)
			if tt.wantErr != "" {
				require.Error(t, sc.Err())
				require.Contains(t, sc.Err().Error(), tt.wantErr)
				return
			}
			require.NoError(t, sc.Err())
			require.JSONEq(t, tt.wantJSON, string(sc.Bytes()))
		})
	}
}
func TestConnect_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMockPing(t)

	mock.ExpectPing()
	err := adapter.Connect()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConnect_GetError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")

	err := adapter.Connect()
	require.Error(t, err)
	require.Contains(t, err.Error(), "connect")
}

func TestPing_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMockPing(t)

	mock.ExpectPing()
	err := adapter.Ping(context.Background())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPing_Error(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMockPing(t)

	mock.ExpectPing().WillReturnError(errors.New("ping failed"))
	err := adapter.Ping(context.Background())
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPing_GetError(t *testing.T) {
	adapter := withFailingDBConnect(t, "get failed")

	err := adapter.Ping(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "get failed")
}

func TestGetTransaction_GetError(t *testing.T) {
	adapter := withFailingDBConnect(t, "get failed")

	tx, err := adapter.GetTransaction()
	require.Error(t, err)
	require.Nil(t, tx)
}

func TestDB_ReturnsInjectedConnection(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	db, err := adapter.DB()
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStmt_Prepare_CacheHit(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		PGDatabase:  defaultMockDB,
		JSONAggType: "json_agg",
		PGCache:     true,
		PGHost:      "localhost",
		PGPort:      5432,
		PGUser:      "u",
		PGSSLMode:   "disable",
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	adapter := New(cfg).(*postgres)
	adapter.conn.SetDatabase(defaultMockDB)
	adapter.conn.InjectDBForTest(adapter.conn.GetURI(defaultMockDB), sqlxDB)
	t.Cleanup(func() { adapter.conn.ResetPoolForTest() })
	adapter.ClearStmt()
	t.Cleanup(adapter.ClearStmt)

	sql := `SELECT 1`
	mock.ExpectPrepare(sql)
	stmt1, err := adapter.Prepare(sqlxDB, sql)
	require.NoError(t, err)
	stmt2, err := adapter.Prepare(sqlxDB, sql)
	require.NoError(t, err)
	require.Same(t, stmt1, stmt2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStmt_PrepareContext_CacheHit(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		PGDatabase:  defaultMockDB,
		JSONAggType: "json_agg",
		PGCache:     true,
		PGHost:      "localhost",
		PGPort:      5432,
		PGUser:      "u",
		PGSSLMode:   "disable",
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	adapter := New(cfg).(*postgres)
	adapter.conn.SetDatabase(defaultMockDB)
	adapter.conn.InjectDBForTest(adapter.conn.GetURI(defaultMockDB), sqlxDB)
	t.Cleanup(func() { adapter.conn.ResetPoolForTest() })
	adapter.ClearStmt()
	t.Cleanup(adapter.ClearStmt)

	ctx := context.Background()
	sql := `SELECT 1`
	mock.ExpectPrepare(sql)
	stmt1, err := adapter.PrepareContext(ctx, sqlxDB, sql)
	require.NoError(t, err)
	stmt2, err := adapter.PrepareContext(ctx, sqlxDB, sql)
	require.NoError(t, err)
	require.Same(t, stmt1, stmt2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAliases(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	require.Equal(t, []string{defaultMockDB}, adapter.Aliases())

	cfg := registryTestConf("tenant-a", "tenant-b")
	adapter = testAdapter(cfg)
	require.Equal(t, []string{"tenant-a", "tenant-b"}, adapter.Aliases())

	cfg = registryTestConf("tenant-a")
	cfg.PGDatabase = ""
	adapter = testAdapter(cfg)
	require.Equal(t, []string{"tenant-a"}, adapter.Aliases())
}

func TestGetDatabase(t *testing.T) {
	t.Parallel()

	adapter, _ := withSQLMock(t)
	adapter.conn.SetDatabase("my-db")
	require.Equal(t, "my-db", adapter.GetDatabase())
}

func TestGetStmt(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	adapter.ClearStmt()
	stmt := adapter.GetStmt()
	require.NotNil(t, stmt)
	require.NotNil(t, stmt.Mtx)
	require.NotNil(t, stmt.PrepareMap)
}

func TestPingAll(t *testing.T) {
	t.Parallel()

	t.Run("default only", func(t *testing.T) {
		t.Parallel()

		adapter, mock := withSQLMockPing(t)
		mock.ExpectPing()
		require.NoError(t, adapter.PingAll(context.Background()))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("default ping fails", func(t *testing.T) {
		t.Parallel()

		adapter, mock := withSQLMockPing(t)
		mock.ExpectPing().WillReturnError(errors.New("ping failed"))
		err := adapter.PingAll(context.Background())
		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("registry pings all aliases", func(t *testing.T) {
		t.Parallel()

		cfg := registryTestConf("tenant-a")
		defaultDB, defaultMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		tenantDB, tenantMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = defaultDB.Close()
			_ = tenantDB.Close()
		})

		adapter := New(cfg).(*postgres)
		adapter.conn.SetDatabase(defaultMockDB)
		adapter.conn.InjectDBForTest(adapter.conn.GetURI(defaultMockDB), sqlx.NewDb(defaultDB, "sqlmock"))
		adapter.conn.InjectDBForTest(adapter.conn.GetURI("tenant-a"), sqlx.NewDb(tenantDB, "sqlmock"))
		t.Cleanup(func() { adapter.conn.ResetPoolForTest() })

		defaultMock.ExpectPing()
		tenantMock.ExpectPing()
		require.NoError(t, adapter.PingAll(context.Background()))
		require.NoError(t, defaultMock.ExpectationsWereMet())
		require.NoError(t, tenantMock.ExpectationsWereMet())
	})

	t.Run("registry alias ping fails", func(t *testing.T) {
		t.Parallel()

		cfg := registryTestConf("tenant-a")
		defaultDB, defaultMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		tenantDB, tenantMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = defaultDB.Close()
			_ = tenantDB.Close()
		})

		adapter := New(cfg).(*postgres)
		adapter.conn.SetDatabase(defaultMockDB)
		adapter.conn.InjectDBForTest(adapter.conn.GetURI(defaultMockDB), sqlx.NewDb(defaultDB, "sqlmock"))
		adapter.conn.InjectDBForTest(adapter.conn.GetURI("tenant-a"), sqlx.NewDb(tenantDB, "sqlmock"))
		t.Cleanup(func() { adapter.conn.ResetPoolForTest() })

		defaultMock.ExpectPing()
		tenantMock.ExpectPing().WillReturnError(errors.New("tenant ping failed"))
		err = adapter.PingAll(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "tenant ping failed")
	})
}

func TestPrepareTxContext(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	mock.ExpectBegin()
	mock.ExpectPrepare(`SELECT 1`)

	tx, err := adapter.GetTransaction()
	require.NoError(t, err)

	stmt, err := adapter.PrepareTxContext(context.Background(), tx, `SELECT 1`)
	require.NoError(t, err)
	require.NotNil(t, stmt)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestQuery_SuccessEmpty(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT 1\) s`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte{}))

	sc := adapter.Query("SELECT 1")
	require.NoError(t, sc.Err())
	require.Equal(t, "[]", string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_SuccessWithData(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT \* FROM users\) s`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[{"id":1}]`)))

	sc := adapter.Query("SELECT * FROM users")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"id":1}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.Query("SELECT 1")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "prepare failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_ScanError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg`).
		ExpectQuery().
		WillReturnError(errors.New("scan failed"))

	sc := adapter.Query("SELECT 1")
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryCtx_WithDBNameKey(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT 1\) s`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))

	sc := adapter.QueryCtx(ctx, "SELECT 1")
	require.NoError(t, sc.Err())
	require.Equal(t, "[1]", string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestInsert_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).AddRow([]byte(`{"name":"alice"}`)))

	sc := adapter.Insert(sql, "alice")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"name":"alice"}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectPrepare(`INSERT INTO`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.Insert(sql, "alice")
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_InvalidSQL(t *testing.T) {
	t.Parallel()

	adapter, _ := withSQLMock(t)

	sc := adapter.Insert("INVALID SQL", "alice")
	require.Error(t, sc.Err())
	require.ErrorIs(t, sc.Err(), ErrNoTableName)
}

func TestDelete_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.Delete(sql, 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectPrepare(`DELETE FROM`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.Delete(sql, 1)
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_ExecError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnError(errors.New("exec failed"))

	sc := adapter.Delete(sql, 1)
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
		ExpectExec().
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.Update(sql, "bob", 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	mock.ExpectPrepare(`UPDATE`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.Update(sql, "bob", 1)
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestShowTable_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT table_schema`).
		ExpectQuery().
		WithArgs("users", "public").
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[{"column_name":"id"}]`)))

	sc := adapter.ShowTable("public", "users")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"column_name":"id"}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_WithStatementCache(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		PGDatabase:  defaultMockDB,
		JSONAggType: "json_agg",
		PGCache:     true,
		PGHost:      "localhost",
		PGPort:      5432,
		PGUser:      "u",
		PGSSLMode:   "disable",
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	adapter := New(cfg).(*postgres)
	adapter.conn.SetDatabase(defaultMockDB)
	adapter.conn.InjectDBForTest(adapter.conn.GetURI(defaultMockDB), sqlxDB)
	t.Cleanup(func() { adapter.conn.ResetPoolForTest() })
	adapter.ClearStmt()
	t.Cleanup(adapter.ClearStmt)
	prep := mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT 1\) s`)
	prep.ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))
	prep.ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))

	sc := adapter.Query("SELECT 1")
	require.NoError(t, sc.Err())

	sc = adapter.Query("SELECT 1")
	require.NoError(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_WithStatementCachePerDatabase(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)
	adapter.getStmts().pgCache = true

	sql := `SELECT json_agg\(s\) FROM \(SELECT 1\) s`
	defaultPrep := defaultMock.ExpectPrepare(sql)
	defaultPrep.ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))
	ctxPrep := ctxMock.ExpectPrepare(sql)
	ctxPrep.ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[2]`)))

	sc := adapter.Query("SELECT 1")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[1]`, string(sc.Bytes()))

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sc = adapter.QueryCtx(ctx, "SELECT 1")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[2]`, string(sc.Bytes()))
	require.NoError(t, defaultMock.ExpectationsWereMet())
	require.NoError(t, ctxMock.ExpectationsWereMet())
}

func TestInsertCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	ctxMock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).AddRow([]byte(`{"name":"alice"}`)))

	sc := adapter.InsertCtx(ctx, sql, "alice")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"name":"alice"}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestDeleteCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	ctxMock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.DeleteCtx(ctx, sql, 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestUpdateCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	ctxMock.ExpectPrepare(`UPDATE "test"."public"."users"`).
		ExpectExec().
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.UpdateCtx(ctx, sql, "bob", 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestQueryCount_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT COUNT\(\*\) FROM users`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(42)))

	sc := adapter.QueryCount(`SELECT COUNT(*) FROM users`)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"count":42}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryCountCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT COUNT\(\*\) FROM users`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(7)))

	sc := adapter.QueryCountCtx(ctx, `SELECT COUNT(*) FROM users`)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"count":7}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestShowTableCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT table_schema`).
		ExpectQuery().
		WithArgs("users", "public").
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[{"column_name":"id"}]`)))

	sc := adapter.ShowTableCtx(ctx, "public", "users")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"column_name":"id"}]`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestShowColumnsCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT table_schema`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[{"table_schema":"public","table_name":"users","column_name":"id"}]`)))

	sc := adapter.ShowColumnsCtx(ctx)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"table_schema":"public","table_name":"users","column_name":"id"}]`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestGetTransaction_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectBegin()
	tx, err := adapter.GetTransaction()
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTransactionCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectBegin()
	tx, err := adapter.GetTransactionCtx(ctx)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestInsertWithTransaction_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectBegin()
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).AddRow([]byte(`{"name":"alice"}`)))

	tx, err := adapter.GetTransaction()
	require.NoError(t, err)
	sc := adapter.InsertWithTransaction(tx, sql, "alice")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"name":"alice"}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteWithTransaction_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectBegin()
	mock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := adapter.GetTransaction()
	require.NoError(t, err)
	sc := adapter.DeleteWithTransaction(tx, sql, 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateWithTransaction_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	mock.ExpectBegin()
	mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
		ExpectExec().
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := adapter.GetTransaction()
	require.NoError(t, err)
	sc := adapter.UpdateWithTransaction(tx, sql, "bob", 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsertValues_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	sql := `INSERT INTO "test"."public"."users"("name","age") VALUES($1,$2),($3,$4)`
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("a", 1, "b", 2).
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).
			AddRow([]byte(`{"name":"a"}`)).
			AddRow([]byte(`{"name":"b"}`)))

	sc := adapter.BatchInsertValues(sql, "a", 1, "b", 2)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"name":"a"},{"name":"b"}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsertValuesCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `INSERT INTO "test"."public"."users"("name","age") VALUES($1,$2),($3,$4)`
	ctxMock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("a", 1, "b", 2).
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).
			AddRow([]byte(`{"name":"a"}`)).
			AddRow([]byte(`{"name":"b"}`)))

	sc := adapter.BatchInsertValuesCtx(ctx, sql, "a", 1, "b", 2)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"name":"a"},{"name":"b"}]`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestBatchInsertCopy_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")

	sc := adapter.BatchInsertCopy(defaultMockDB, "public", "users", []string{"name"}, "alice")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "connect")
}

func TestBatchInsertCopyCtx_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)

	sc := adapter.BatchInsertCopyCtx(ctx, contextMockDB, "public", "users", []string{"name"}, "alice")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "connect")
}

func TestQuery_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	sc := adapter.Query("SELECT 1")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "connect")
}

func TestQueryCtx_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	mock.ExpectPrepare(`SELECT json_agg`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.QueryCtx(context.Background(), "SELECT 1")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "prepare failed")
}

func TestQueryCtx_ScanError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	mock.ExpectPrepare(`SELECT json_agg`).
		ExpectQuery().
		WillReturnError(errors.New("scan failed"))

	sc := adapter.QueryCtx(context.Background(), "SELECT 1")
	require.Error(t, sc.Err())
}

func TestQueryCount_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	mock.ExpectPrepare(`SELECT COUNT`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.QueryCount(`SELECT COUNT(*) FROM users`)
	require.Error(t, sc.Err())
}

func TestQueryCount_ScanError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	mock.ExpectPrepare(`SELECT COUNT`).
		ExpectQuery().
		WillReturnError(errors.New("scan failed"))

	sc := adapter.QueryCount(`SELECT COUNT(*) FROM users`)
	require.Error(t, sc.Err())
}

func TestQueryCountCtx_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT COUNT`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.QueryCountCtx(ctx, `SELECT COUNT(*) FROM users`)
	require.Error(t, sc.Err())
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestInsert_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	sc := adapter.Insert(`INSERT INTO "test"."public"."users"("name") VALUES($1)`, "alice")
	require.Error(t, sc.Err())
}

func TestInsertCtx_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sc := adapter.InsertCtx(ctx, `INSERT INTO "test"."public"."users"("name") VALUES($1)`, "alice")
	require.Error(t, sc.Err())
}

func TestDelete_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	sc := adapter.Delete(`DELETE FROM "test"."public"."users" WHERE "id"=$1`, 1)
	require.Error(t, sc.Err())
}

func TestDeleteCtx_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sc := adapter.DeleteCtx(ctx, `DELETE FROM "test"."public"."users" WHERE "id"=$1`, 1)
	require.Error(t, sc.Err())
}

func TestUpdate_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	sc := adapter.Update(`UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`, "bob", 1)
	require.Error(t, sc.Err())
}

func TestUpdateCtx_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sc := adapter.UpdateCtx(ctx, `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`, "bob", 1)
	require.Error(t, sc.Err())
}

func TestGetTransactionCtx_GetError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	tx, err := adapter.GetTransactionCtx(ctx)
	require.Error(t, err)
	require.Nil(t, tx)
}

func TestBatchInsertValues_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	sc := adapter.BatchInsertValues(`INSERT INTO "test"."public"."users"("name") VALUES($1)`, "alice")
	require.Error(t, sc.Err())
}

func TestBatchInsertValues_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.BatchInsertValues(sql, "alice")
	require.Error(t, sc.Err())
}

func TestBatchInsertValues_QueryError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("alice").
		WillReturnError(errors.New("query failed"))

	sc := adapter.BatchInsertValues(sql, "alice")
	require.Error(t, sc.Err())
}

func TestBatchInsertValues_ScanError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json", "extra_col"}).AddRow([]byte(`{"name":"alice"}`), "x"))

	sc := adapter.BatchInsertValues(sql, "alice")
	require.Error(t, sc.Err())
}

func TestBatchInsertValuesCtx_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sc := adapter.BatchInsertValuesCtx(ctx, `INSERT INTO "test"."public"."users"("name") VALUES($1)`, "alice")
	require.Error(t, sc.Err())
}

func TestDelete_RowsAffectedError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnResult(errRowsAffectedResult{})

	sc := adapter.Delete(sql, 1)
	require.Error(t, sc.Err())
}

func TestUpdate_RowsAffectedError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
		ExpectExec().
		WithArgs("bob", 1).
		WillReturnResult(errRowsAffectedResult{})

	sc := adapter.Update(sql, "bob", 1)
	require.Error(t, sc.Err())
}

func TestDelete_ReturningByteColumn(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1 RETURNING "name"`
	mock.ExpectPrepare(`DELETE FROM`).
		ExpectQuery().
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow([]byte("alice")))

	sc := adapter.Delete(sql, 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"name":"alice"}]`, string(sc.Bytes()))
}

func TestDbFromCtx_AddDatabaseToPoolFailure(t *testing.T) {
	adapter := withFailingDBConnect(t, "pool add failed")
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "missing-db")

	sc := adapter.QueryCtx(ctx, "SELECT 1")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "pool add failed")
}
