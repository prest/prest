package postgres

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func testAdapter() *Postgres {
	return &Postgres{}
}

func withPrestConf(t *testing.T, cfg *config.Prest) {
	t.Helper()
	old := config.PrestConf
	config.PrestConf = cfg
	t.Cleanup(func() { config.PrestConf = old })
}

func defaultTestConf() *config.Prest {
	return &config.Prest{
		PGDatabase:  "test",
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/?_groupby=name,age", nil)
	require.NoError(t, err)
	clause := adapter.GroupByClause(req)
	require.Contains(t, clause, "GROUP BY")
	require.Contains(t, clause, `"name"`)
	require.Contains(t, clause, `"age"`)
}

func TestJoinByRequest(t *testing.T) {
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
	adapter := testAdapter()

	sql, err := adapter.SelectFields([]string{"name", "age"})
	require.NoError(t, err)
	require.Contains(t, sql, `"name"`)
	require.Contains(t, sql, `"age"`)

	_, err = adapter.SelectFields(nil)
	require.Error(t, err)
}

func TestCountByRequest(t *testing.T) {
	withPrestConf(t, defaultTestConf())
	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/public/test?_count=true", nil)
	require.NoError(t, err)
	count, err := adapter.CountByRequest(req)
	require.NoError(t, err)
	require.Contains(t, count, "COUNT")
}

func TestPaginateIfPossible(t *testing.T) {
	withPrestConf(t, defaultTestConf())
	adapter := testAdapter()

	req, err := http.NewRequest(http.MethodGet, "/public/test?_page=1&_page_size=10", nil)
	require.NoError(t, err)
	page, err := adapter.PaginateIfPossible(req)
	require.NoError(t, err)
	require.Contains(t, page, "LIMIT")
	require.Contains(t, page, "OFFSET")
}

func TestDatabaseClause(t *testing.T) {
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, defaultTestConf())
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
	withPrestConf(t, permissionTestConf())
	adapter := testAdapter()

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

	withPrestConf(t, defaultTestConf())
	got := adapter.TablePermissions("any_table", "read", "")
	require.True(t, got)
}

func TestFieldsPermissions(t *testing.T) {
	withPrestConf(t, permissionTestConf())
	adapter := testAdapter()

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
}

func TestFieldsByPermission(t *testing.T) {
	withPrestConf(t, permissionTestConf())

	fields := fieldsByPermission("test_fields_access", "read", "")
	require.Equal(t, []string{"name", "surname"}, fields)

	fields = fieldsByPermission("test_write_and_delete_access", "read", "foo_read")
	require.Equal(t, []string{"*"}, fields)

	fields = fieldsByPermission("no_user_write_table", "write", "foo_read")
	require.Equal(t, []string{"name"}, fields)
}
