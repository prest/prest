package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
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

func TestParseInsertRequest(t *testing.T) {

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

func TestDistinctClause(t *testing.T) {

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

func TestJoinByRequest(t *testing.T) {

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

func TestOrderByRequest(t *testing.T) {

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

	adapter := testAdapter()

	sql, err := adapter.SelectFields([]string{"name", "age"})
	require.NoError(t, err)
	require.Contains(t, sql, `"name"`)
	require.Contains(t, sql, `"age"`)

	_, err = adapter.SelectFields(nil)
	require.Error(t, err)
}

func TestCountByRequest(t *testing.T) {

	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/public/test?_count=true", nil)
	require.NoError(t, err)
	count, err := adapter.CountByRequest(req)
	require.NoError(t, err)
	require.Contains(t, count, "COUNT")
}

func TestPaginateIfPossible(t *testing.T) {

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
	cols, err := normalizeAll([]string{"name", "max:age"})
	require.NoError(t, err)
	require.Equal(t, []string{"name", `MAX("age")`}, cols)

	_, err = normalizeAll([]string{"bad:col"})
	require.Error(t, err)
}

func TestDatabaseClause(t *testing.T) {

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

func TestTablePermissions(t *testing.T) {
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
			got := adapter.TablePermissions(tc.table, tc.permission, tc.userName)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestTablePermissionsUnrestrict(t *testing.T) {
	cfg := permissionTestConf()
	cfg.AccessConf.Restrict = false
	adapter := testAdapter(cfg)

	got := adapter.TablePermissions("any_table", "read", "")
	require.True(t, got)
}

func TestFieldsPermissions(t *testing.T) {
	adapter := testAdapter(permissionTestConf())

	req, err := http.NewRequest(http.MethodGet, "/public/test?_select=name,surname", nil)
	require.NoError(t, err)
	fields, err := adapter.FieldsPermissions(req, "test_fields_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name", "surname"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "test_fields_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name", "surname"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_select=name,surname", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "test_fields_access", "delete", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name", "surname"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_select=name", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "test_readonly_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"name"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "test_readonly_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"*"}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test?_select=max:age&_groupby=status", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "test_readonly_access", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{`MAX("age")`}, fields)

	req, err = http.NewRequest(http.MethodGet, "/public/test", nil)
	require.NoError(t, err)
	fields, err = adapter.FieldsPermissions(req, "no_user_write_table", "write", "foo_read")
	require.NoError(t, err)
	require.Equal(t, []string{"name"}, fields)
}

func TestFieldsByPermission(t *testing.T) {
	adapter := testAdapter(permissionTestConf())

	fields := adapter.fieldsByPermission("test_fields_access", "read", "")
	require.Equal(t, []string{"name", "surname"}, fields)

	fields = adapter.fieldsByPermission("test_write_and_delete_access", "read", "foo_read")
	require.Equal(t, []string{"*"}, fields)

	fields = adapter.fieldsByPermission("no_user_write_table", "write", "foo_read")
	require.Equal(t, []string{"name"}, fields)
}

func Test_isTopLevelOrSeparator(t *testing.T) {
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

func Test_postgres_whereKeyAndValue(t *testing.T) {
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
