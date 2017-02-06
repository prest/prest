package postgres

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/statements"
)

func TestWhereByRequest(t *testing.T) {
	var testCases = []struct {
		description    string
		url            string
		expectedSQL    []string
		expectedValues []string
		err            error
	}{
		{"Where by request without paginate", "/databases?dbname=$eq.prest&test=$eq.cool", []string{"dbname = $", "test = $", " AND "}, []string{"prest", "cool"}, nil},
		{"Where by request with spaced values", "/prest/public/test5?name=$eq.prest tester", []string{"name = $"}, []string{"prest tester"}, nil},
		{"Where by request with jsonb field", "/prest/public/test_jsonb_bug?name=$eq.goku&data->>description:jsonb=$eq.testing", []string{"name = $", "data->>'description' = $", " AND "}, []string{"goku", "testing"}, nil},
		{"Where by request with spaced values", "/prest/public/test5?name=$eq.prest.txt tester", []string{"name = $"}, []string{"prest.txt tester"}, nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		where, values, err := WhereByRequest(req, 1)
		if err != nil {
			t.Errorf("expected no errors in where by request, got %v", err)
		}

		for _, sql := range tc.expectedSQL {
			if !strings.Contains(where, sql) {
				t.Errorf("expected %s in %s, but not was!", sql, where)
			}
		}

		expectedValuesSTR := strings.Join(tc.expectedValues, " ")
		for _, value := range values {
			if !strings.Contains(expectedValuesSTR, value.(string)) {
				t.Errorf("expected %s in %s", value, expectedValuesSTR)
			}
		}
	}
}

func TestInvalidWhereByRequest(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
	}{
		{"Where by request without jsonb key", "/prest/public/test_jsonb_bug?name=$eq.nuveo&data->>description:bla"},
		{"Where by request with jsonb field invalid", "/prest/public/test_jsonb_bug?name=$eq.nuveo&data->>0description:jsonb=$eq.bla"},
		{"Where by request with field invalid", "/prest/public/test?0name=$eq.prest"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		where, values, err := WhereByRequest(req, 1)
		if err == nil {
			t.Errorf("expected errors in where by request, got %v", err)
		}

		if where != "" {
			t.Errorf("expected empty `where`, got %v", where)
		}

		if values != nil {
			t.Errorf("expected empty `values`, got %v", values)
		}
	}
}

func TestQuery(t *testing.T) {
	var response []byte
	var err error

	var testCases = []struct {
		description string
		sql         string
		param       bool
		jsonMinLen  int
		err         error
	}{
		{"Query execution", "SELECT schema_name FROM information_schema.schemata ORDER BY schema_name ASC", false, 1, nil},
		{"Query execution 2", "SELECT number FROM prest.public.test2 ORDER BY number ASC", false, 1, nil},
		{"Query execution with params", "SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC", true, 1, nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		if tc.param {
			response, err = Query(tc.sql, "public")
		} else {
			response, err = Query(tc.sql)
		}

		if err != tc.err {
			t.Errorf("expected no errors, but got %s", err)
		}

		if len(response) < tc.jsonMinLen {
			t.Errorf("expected valid json response, but got %v", string(response))
		}
	}
}

func TestInvalidQuery(t *testing.T) {
	var testCases = []struct {
		description string
		sql         string
	}{
		{"Query with invalid characters", "SELECT ~~, ``, ˜ schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC"},
		{"Query with invalid clause", "0SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		response, err := Query(tc.sql, "public")

		if err == nil {
			t.Error("expected errors, but got nil")
		}

		if response != nil {
			t.Errorf("expected no response, but got %s", string(response))
		}
	}
}

func TestPaginateIfPossible(t *testing.T) {
	var testCase = []struct {
		description string
		url         string
		expected    string
		err         error
	}{
		{"Paginate if possible", "/databases?dbname=prest&test=cool&_page=1&_page_size=20", "LIMIT 20 OFFSET(1 - 1) * 20", nil},
		{"Invalid Paginate if possible", "/databases?dbname=prest&test=cool", "", nil},
	}

	for _, tc := range testCase {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, but got %s", err)
		}

		sql, err := PaginateIfPossible(req)
		if err != nil {
			t.Errorf("expected no errors, but got %s", err)
		}

		if !strings.Contains(tc.expected, sql) {
			t.Errorf("expected %s in %s, but not was!", tc.expected, sql)
		}
	}
}

func TestInvalidPaginateIfPossible(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
	}{
		{"Paginate with invalid page value", "/databases?dbname=prest&test=cool&_page=X&_page_size=20"},
		{"Paginate with invalid page size value", "/databases?dbname=prest&test=cool&_page=1&_page_size=K"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, but got %s", err)
		}

		sql, err := PaginateIfPossible(req)
		if err == nil {
			t.Errorf("expected errors, but got %s", err)
		}

		if sql != "" {
			t.Errorf("expected empty sql, but got: %s", sql)
		}
	}
}

func TestInsert(t *testing.T) {
	var testCases = []struct {
		description string
		key         string
		value       string
		db          string
		schema      string
		table       string
		result      interface{}
		primaryKey  string
	}{
		{"Insert data into a table with primary key", "name", "prest-test-insert", "prest", "public", "test4", 1.0, "id"},
		{"Insert data into a table with primary key named nuveo", "name", "prest-test-insert", "prest", "public", "test6", 1.0, "nuveo"},
		{"Insert data into a table without primary key", "name", "prest-test-insert", "prest", "public", "test6", nil, ""},
	}

	t.Log("Insert data with more columns in table")
	m := make(map[string]interface{}, 0)
	m["name"] = "prest-test-insert"
	m["celphone"] = "88888888888"
	r := api.Request{
		Data: m,
	}

	jsonByte, err := Insert("prest", "public", "test5", r)
	if err != nil {
		t.Errorf("expected no errors, but got %s", err)
	}
	if len(jsonByte) < 1 {
		t.Errorf("expected valid response body, but got %s", string(jsonByte))
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		m := make(map[string]interface{}, 0)
		m[tc.key] = tc.value
		r := api.Request{
			Data: m,
		}

		jsonByte, err := Insert(tc.db, tc.schema, tc.table, r)
		if err != nil {
			t.Errorf("expected no errors, but got %s", err)
		}
		if len(jsonByte) < 1 {
			t.Errorf("expected valid response body, but got %s", string(jsonByte))
		}

		var toJSON map[string]interface{}
		err = json.Unmarshal(jsonByte, &toJSON)
		if err != nil {
			t.Errorf("expected no errors, but got %s", err)
		}

		if tc.primaryKey != "" && toJSON[tc.primaryKey] != tc.result {
			t.Errorf("expected %v in result, got %v", toJSON[tc.primaryKey], tc.result)
		}
	}
}

func TestInvalidInsert(t *testing.T) {
	var testCases = []struct {
		description string
		key         string
		value       string
		db          string
		schema      string
		table       string
	}{
		{"Insert invalid data into a table with primary key", "prest", "prest-test-insert", "prest", "public", "test6"},
		{"Insert data into a table with contraints", "name", "prest", "prest", "public", "test3"},
		{"Insert data into a database invalid", "name", "prest", "0prest", "public", "test3"},
		{"Insert data into a schema invalid", "name", "prest", "prest", "0public", "test3"},
		{"Insert data into a table invalid", "name", "prest", "prest", "public", "0test3"},
		{"Insert data into a request invalid", "0name", "prest", "prest", "public", "test3"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		m := make(map[string]interface{}, 0)
		m[tc.key] = tc.value
		r := api.Request{
			Data: m,
		}

		jsonByte, err := Insert(tc.db, tc.schema, tc.table, r)
		if err == nil {
			t.Errorf("expected no errors, but got %s", err)
		}

		if len(jsonByte) > 0 {
			t.Errorf("expected invalid response body, but got %s", string(jsonByte))
		}
	}
}

func TestDelete(t *testing.T) {
	var testCases = []struct {
		description string
		db          string
		schema      string
		table       string
		partialSQL  string
		values      []interface{}
	}{
		{"Try Delete data from invalid database", "0prest", "public", "test", "name=$1", []interface{}{"nuveo"}},
		{"Try Delete data from invalid schema", "prest", "0public", "test", "name=$1", []interface{}{"nuveo"}},
		{"Try Delete data from invalid table", "prest", "public", "0test", "name=$1", []interface{}{"nuveo"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		response, err := Delete(tc.db, tc.schema, tc.table, tc.partialSQL, tc.values)
		if err == nil {
			t.Errorf("expected error, but got: %s", err)
		}

		if len(response) > 0 {
			t.Errorf("expected empty response body, but got %s", string(response))
		}
	}

	t.Log("Delete data from table")
	response, err := Delete("prest", "public", "test", "name=$1", []interface{}{"nuveo"})
	if err != nil {
		t.Errorf("expected no error, but got: %s", err)
	}

	if len(response) < 1 {
		t.Errorf("expected response body, but got %s", string(response))
	}
}

func TestUpdate(t *testing.T) {
	var testCases = []struct {
		description string
		db          string
		schema      string
		table       string
		partialSQL  string
		values      []interface{}
	}{
		{"Update data into an invalid database", "0prest", "public", "test3", "name=$1", []interface{}{"prest tester"}},
		{"Update data into an invalid schema", "prest", "0public", "test3", "name=$1", []interface{}{"prest tester"}},
		{"Update data into an invalid table", "prest", "public", "0test3", "name=$1", []interface{}{"prest tester"}},
	}
	m := make(map[string]interface{}, 0)
	m["name"] = "prest"

	r := api.Request{
		Data: m,
	}

	t.Log("Update data into a table")
	response, err := Update("prest", "public", "test", "name=$1", []interface{}{"prest"}, r)
	if err != nil {
		t.Errorf("expected no errors, but got: %s", err)
	}

	if len(response) < 1 {
		t.Errorf("expected a valid response body, but got %s", string(response))
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		response, err := Update(tc.db, tc.schema, tc.table, tc.partialSQL, tc.values, r)
		if err == nil {
			t.Errorf("expected error, but got: %s", err)
		}

		if len(response) > 0 {
			t.Errorf("expected empty response body, but got %s", string(response))
		}
	}
}

func TestChkInvaidIdentifier(t *testing.T) {
	var testCases = []struct {
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
		{"_123456789_123456789_123456789_123456789_123456789_123456789_12345", true},
	}

	for _, tc := range testCases {
		result := chkInvalidIdentifier(tc.in)
		if result != tc.out {
			t.Errorf("expected %v, got %v", tc.out, result)
		}
	}
}

func TestJoinByRequest(t *testing.T) {
	var testCases = []struct {
		description     string
		url             string
		expectedValues  []string
		testEmptyResult bool
	}{
		{"Join by request", "/prest/public/test?_join=inner:test2:test2.name:$eq:test.name", []string{"INNER JOIN", "test2 ON ", "test2.name = test.name"}, false},
		{"Join empty params", "/prest/public/test?_join", []string{}, true},
		{"Join missing param", "/prest/public/test?_join=inner:test2:test2.name:$eq", []string{}, true},
		{"Join invalid operator", "/prest/public/test?_join=inner:test2:test2.name:notexist:test.name", []string{}, true},
		{"Join invalid fields", "/prest/public/test?_join=inner:0test2:test2.name:notexist:test.name", []string{}, true},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, got %v", err)
		}

		join, err := JoinByRequest(req)
		if tc.testEmptyResult {
			if join != nil {
				t.Errorf("expected empty response, but got: %v", join)
			}
		} else {
			if err != nil {
				t.Errorf("expected no errors, but got: %v", err)
			}

			joinSQL := strings.Join(join, " ")

			for _, sql := range tc.expectedValues {
				if !strings.Contains(joinSQL, sql) {
					t.Errorf("expected %s in %s, but no was!", sql, joinSQL)
				}
			}
		}
	}

	t.Log("Join with where")
	var expectedSQL = []string{"name = $", "data->>'description' = $", " AND "}
	var expectedValues = []string{"nuveo", "bla"}

	r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:test2:test2.name:$eq:test.name&name=$eq.nuveo&data->>description:jsonb=$eq.bla", nil)
	if err != nil {
		t.Errorf("expected no errorn on New Request, got %v", err)
	}

	join, err := JoinByRequest(r)
	if err != nil {
		t.Errorf("expected no errors, but got: %v", err)
	}

	joinStr := strings.Join(join, " ")

	if !strings.Contains(joinStr, " INNER JOIN test2 ON test2.name = test.name") {
		t.Errorf("expected %s in INNER JOIN test2 ON test2.name = test.name, but no was!", joinStr)
	}

	where, values, err := WhereByRequest(r, 1)
	if err != nil {
		t.Errorf("expected no errors, got: %v", err)
	}

	for _, sql := range expectedSQL {
		if !strings.Contains(where, sql) {
			t.Errorf("expected %s in %s, but not was!", sql, where)
		}
	}

	expectedValuesSTR := strings.Join(expectedValues, " ")
	for _, value := range values {
		if !strings.Contains(expectedValuesSTR, value.(string)) {
			t.Errorf("expected %s in %s", value, expectedValuesSTR)
		}
	}
}

func TestCountFields(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		expectedSQL string
		testError   bool
	}{
		{"Count fields from table", "/prest/public/test5?_count=celphone", "SELECT COUNT(celphone) FROM", false},
		{"Count all from table", "/prest/public/test5?_count=*", "SELECT COUNT(*) FROM", false},
		{"Count with empty params", "/prest/public/test5?_count=", "", false},
		{"Count with invalid columns", "/prest/public/test5?_count=celphone,0name", "", true},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, got: %v", err)
		}

		sql, err := CountByRequest(req)
		if tc.testError {
			if err == nil {
				t.Error("expected errors, but no was!")
			}

			if sql != "" {
				t.Errorf("expected empty sql, but got: %s", sql)
			}
		} else {
			if err != nil {
				t.Errorf("expected no errors, but got: %v", err)
			}

			if !strings.Contains(sql, tc.expectedSQL) {
				t.Errorf("expected %s in %s", tc.expectedSQL, sql)
			}
		}
	}

}

func TestDatabaseClause(t *testing.T) {
	var testCases = []struct {
		description   string
		url           string
		queryExpected string
	}{
		{"Return appropriate SELECT clause", "/databases", fmt.Sprintf(statements.DatabasesSelect, statements.FieldDatabaseName)},
		{"Return appropriate COUNT clause", "/databases?_count=*", fmt.Sprintf(statements.DatabasesSelect, statements.FieldCountDatabaseName)},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		r, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, got: %v", err)
		}

		query := DatabaseClause(r)
		if query != tc.queryExpected {
			t.Errorf("query unexpected, got: %s", query)
		}
	}

}

func TestSchemaClause(t *testing.T) {
	var testCases = []struct {
		description   string
		url           string
		queryExpected string
	}{
		{"Return appropriate SELECT clause", "/schemas", fmt.Sprintf(statements.SchemasSelect, statements.FieldSchemaName)},
		{"Return appropriate COUNT clause", "/schemas?_count=*", fmt.Sprintf(statements.SchemasSelect, statements.FieldCountSchemaName)},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		r, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, got: %v", err)
		}

		query := SchemaClause(r)
		if query != tc.queryExpected {
			t.Errorf("query unexpected, got: %s", query)
		}
	}
}

func TestGetQueryOperator(t *testing.T) {
	var testCases = []struct {
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
		{"$notnull", "IS NOT NULL"},
		{"$null", "IS NULL"},
	}

	for _, tc := range testCases {
		t.Log(fmt.Sprintf("Query operator %s", tc.in))
		op, err := GetQueryOperator(tc.in)
		if err != nil {
			t.Errorf("expected no errors, got: %v", err)
		}

		if op != tc.out {
			t.Errorf("expected %s, got: %s", tc.out, op)
		}
	}

	t.Log("Invalid query operator")
	op, err := GetQueryOperator("!lol")
	if err == nil {
		t.Errorf("expected errors, got: %v", err)
	}

	if op != "" {
		t.Errorf("expected empty op, got: %s", op)
	}
}

func TestOrderByRequest(t *testing.T) {
	t.Log("Query ORDER BY")
	var expectedSQL = []string{"ORDER BY", "name", "number DESC"}

	r, err := http.NewRequest("GET", "/prest/public/test?_order=name,-number", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, got: %v", err)
	}

	order, err := OrderByRequest(r)
	if err != nil {
		t.Errorf("expected no errors on OrderByRequest, got: %v", err)
	}
	for _, sql := range expectedSQL {
		if !strings.Contains(order, sql) {
			t.Errorf("expected %s in %s, but no was!", sql, order)
		}
	}

	t.Log("Query ORDER BY empty")
	r, err = http.NewRequest("GET", "/prest/public/test?_order=", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, got: %v", err)
	}

	order, err = OrderByRequest(r)
	if err != nil {
		t.Errorf("expected no errors on OrderByRequest, got: %v", err)
	}

	if order != "" {
		t.Errorf("expected order empty, got: %s", order)
	}

	t.Log("Query ORDER BY invalid column")
	r, err = http.NewRequest("GET", "/prest/public/test?_order=0name", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, got: %v", err)
	}

	order, err = OrderByRequest(r)
	if err == nil {
		t.Errorf("expected errors on OrderByRequest, got: %v", err)
	}

	if order != "" {
		t.Errorf("expected order empty, got: %s", order)
	}
}

func TestTablePermissions(t *testing.T) {
	var testCases = []struct {
		description string
		table       string
		permission  string
		out         bool
	}{
		{"Read", "test_readonly_access", "read", true},
		{"Try to read without permission", "test_write_and_delete_access", "read", false},
		{"Write", "test_write_and_delete_access", "write", true},
		{"Try to write without permission", "test_readonly_access", "write", false},
		{"Delete", "test_write_and_delete_access", "delete", true},
		{"Try to delete without permission", "test_readonly_access", "delete", false},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		p := TablePermissions(tc.table, tc.permission)

		if p != tc.out {
			t.Errorf("expected %v, got %v", tc.out, p)
		}
	}

}

func TestFieldsPermissions(t *testing.T) {
	var testCases = []struct {
		description string
		table       string
		fields      []string
		permission  string
		resultLen   int
	}{
		{"Read valid field", "test_list_only_id", []string{"id"}, "read", 1},
		{"Read invalid field", "test_list_only_id", []string{"name"}, "read", 0},
		{"Read non existing field", "test_list_only_id", []string{"non_existing_field"}, "read", 0},
		{"Select with *", "test_list_only_id", []string{"*"}, "read", 1},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		fields := FieldsPermissions(tc.table, tc.fields, tc.permission)
		if len(fields) != tc.resultLen {
			t.Errorf("expected %d, got: %d - %v", tc.resultLen, len(fields), fields)
		}
	}
}

func TestRestrictFalse(t *testing.T) {
	config.PREST_CONF.AccessConf.Restrict = false

	t.Log("Read unrestrict")
	fields := FieldsPermissions("test_list_only_id", []string{"*"}, "read")
	if fields[0] != "*" {
		t.Errorf("expected '*', got: %s", fields[0])
	}

	t.Log("Restrict disabled")
	p := TablePermissions("test_readonly_access", "delete")
	if !p {
		t.Errorf("expected %v, got: %v", p, !p)
	}
}

func TestSelectFields(t *testing.T) {
	var testCases = []struct {
		description string
		fields      []string
		expectedSQL string
	}{
		{"One field", []string{"test"}, "SELECT test FROM"},
		{"More field", []string{"test", "test02"}, "SELECT test,test02 FROM"},
	}
	var testErrorCases = []struct {
		description string
		fields      []string
		expectedSQL string
	}{
		{"Invalid fields", []string{"0test", "test02"}, ""},
		{"Empty fields", []string{}, ""},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		sql, err := SelectFields(tc.fields)
		if err != nil {
			t.Errorf("expected no errors, but got: %v", err)
		}

		if sql != tc.expectedSQL {
			t.Errorf("expected '%s', got: '%s'", tc.expectedSQL, sql)
		}
	}

	for _, tc := range testErrorCases {
		t.Log(tc.description)
		sql, err := SelectFields(tc.fields)
		if err == nil {
			t.Errorf("expected errors, but got: %v", err)
		}

		if sql != tc.expectedSQL {
			t.Errorf("expected '%s', got: '%s'", tc.expectedSQL, sql)
		}
	}
}

func TestColumnsByRequest(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		expectedSQL string
	}{
		{"Select fields from table", "/prest/public/test5?_select=celphone", "celphone"},
		{"Select all from table", "/prest/public/test5?_select=*", "*"},
		{"Select with empty '_select' field", "/prest/public/test5?_select=", "*"},
		{"Select with more columns", "/prest/public/test5?_select=celphone,battery", "celphone,battery"},
	}
	for _, tc := range testCases {
		t.Log(tc.description)
		r, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, but got: %v", err)
		}

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		if selectStr != tc.expectedSQL {
			t.Errorf("expected %s, got: %s", tc.expectedSQL, selectStr)
		}
	}
}
