package postgres

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/prest/adapters/postgres/connection"

	"bytes"

	"github.com/prest/config"
	"github.com/prest/statements"
)

func TestParseInsertRequest(t *testing.T) {
	m := make(map[string]interface{})
	m["name"] = "prest"
	mc := make(map[string]interface{})
	mc["test"] = "prest"
	mc["dbname"] = "prest"

	var testCases = []struct {
		description      string
		body             map[string]interface{}
		expectedColNames []string
		expectedValues   []string
		err              error
	}{
		{"insert by request more than one field", mc, []string{"dbname", "test"}, []string{"prest", "prest"}, nil},
		{"insert by request one field", m, []string{"name"}, []string{"prest"}, nil},
		{"insert by request empty body", nil, nil, nil, ErrBodyEmpty},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		body, err := json.Marshal(tc.body)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}
		req, err := http.NewRequest("POST", "/", bytes.NewReader(body))
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		colsNames, _, values, err := ParseInsertRequest(req)
		if err != tc.err {
			t.Errorf("expected errors %v in where by request, got %v", tc.err, err)
		}

		for _, sql := range tc.expectedColNames {
			if !strings.Contains(colsNames, sql) {
				t.Errorf("expected %s in %s, but not was!", sql, colsNames)
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

func TestSetByRequest(t *testing.T) {
	m := make(map[string]interface{})
	m["name"] = "prest"
	mc := make(map[string]interface{})
	mc["test"] = "prest"
	mc["dbname"] = "prest"
	ma := make(map[string]interface{})
	ma["c.name"] = "prest"

	var testCases = []struct {
		description    string
		body           map[string]interface{}
		expectedSQL    []string
		expectedValues []string
		err            error
	}{
		{"set by request more than one field", mc, []string{`"dbname"=$`, `"test"=$`, ", "}, []string{"prest", "prest"}, nil},
		{"set by request one field", m, []string{`"name"=$`}, []string{"prest"}, nil},
		{"set by request alias", ma, []string{`"c".`, `"name"=$`}, []string{"prest"}, nil},
		{"set by request empty body", nil, nil, nil, ErrBodyEmpty},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		body, err := json.Marshal(tc.body)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}
		req, err := http.NewRequest("PUT", "/", bytes.NewReader(body))
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		setSyntax, values, err := SetByRequest(req, 1)
		if err != tc.err {
			t.Errorf("expected errors %v in where by request, got %v", tc.err, err)
		}

		for _, sql := range tc.expectedSQL {
			if !strings.Contains(setSyntax, sql) {
				t.Errorf("expected %s in %s, but not was!", sql, setSyntax)
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

func TestWhereByRequest(t *testing.T) {
	var testCases = []struct {
		description    string
		url            string
		expectedSQL    []string
		expectedValues []string
		err            error
	}{
		{"Where by request without paginate", "/databases?dbname=$eq.prest&test=$eq.cool", []string{`"dbname" = $`, `"test" = $`, " AND "}, []string{"prest", "cool"}, nil},
		{"Where by request with alias", "/databases?dbname=$eq.prest&c.test=$eq.cool", []string{`"dbname" = $`, `"c".`, `"test" = $`, " AND "}, []string{"prest", "cool"}, nil},
		{"Where by request with spaced values", "/prest/public/test5?name=$eq.prest tester", []string{`"name" = $`}, []string{"prest tester"}, nil},
		{"Where by request with jsonb field", "/prest/public/test_jsonb_bug?name=$eq.goku&data->>description:jsonb=$eq.testing", []string{`"name" = $`, `"data"->>'description' = $`, " AND "}, []string{"goku", "testing"}, nil},
		{"Where by request with dot values", "/prest/public/test5?name=$eq.prest.txt tester", []string{`"name" = $`}, []string{"prest.txt tester"}, nil},
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

func TestGroupByClause(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		expectedSQL string
		emptyCase   bool
	}{
		{"Group by clause with one field", "/prest/public/test5?_groupby=celphone", `GROUP BY "celphone"`, false},
		{"Group by clause with two fields", "/prest/public/test5?_groupby=celphone,name", `GROUP BY "celphone","name"`, false},
		{"Group by clause with two fields", "/prest/public/test5?_groupby=c.celphone,c.name", `GROUP BY "c"."celphone","c"."name"`, false},
		{"Group by clause without fields", "/prest/public/test5?_groupby=", "", true},

		// having tests
		{"Group by clause with having clause", "/prest/public/test5?_groupby=celphone->>having:sum:salary:$gt:500", `GROUP BY "celphone" HAVING SUM("salary") > 500`, false},
		{"Group by clause with having clause", "/prest/public/test5?_groupby=c.celphone->>having:sum:salary:$gt:500", `GROUP BY "c"."celphone" HAVING SUM("salary") > 500`, false},

		// having errors, but continue with group by
		{"Group by clause with wrong having clause (insufficient params)", "/prest/public/test5?_groupby=celphone->>having:sum:salary", `GROUP BY "celphone"`, false},
		{"Group by clause with wrong having clause (wrong query operator)", "/prest/public/test5?_groupby=celphone->>having:sum:salary:$at:500", `GROUP BY "celphone"`, false},
		{"Group by clause with wrong having clause (wrong group func)", "/prest/public/test5?_groupby=celphone->>having:sun:salary:$gt:500", `GROUP BY "celphone"`, false},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		groupBySQL := GroupByClause(req)

		if !tc.emptyCase && groupBySQL == "" {
			t.Error("expected groupBySQL, got empty string")
		}

		if tc.emptyCase && groupBySQL != "" {
			t.Errorf("expected empty, got %v", groupBySQL)
		}

		if groupBySQL != tc.expectedSQL {
			t.Errorf("expected %s, got %s", tc.expectedSQL, groupBySQL)
		}
	}
}

func TestEmptyTable(t *testing.T) {
	sc := Query("SELECT * FROM test_empty_table")
	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}
	if !bytes.Equal(sc.Bytes(), []byte("[]")) {
		t.Fatalf("Query response returned '%v', expected '[]'", string(sc.Bytes()))
	}
}

func TestQuery(t *testing.T) {
	var sc Scanner

	var testCases = []struct {
		description string
		sql         string
		param       bool
		jsonMinLen  int
		err         error
	}{
		{"Query execution", "SELECT schema_name FROM information_schema.schemata ORDER BY schema_name ASC", false, 1, nil},
		{"Query execution 2", "SELECT number FROM prest.public.test2 ORDER BY number ASC", false, 1, nil},
		{"Query execution with quotes", `SELECT "number" FROM "prest"."public"."test2" ORDER BY "number" ASC`, false, 1, nil},
		{"Query execution with params", "SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC", true, 1, nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		if tc.param {
			sc = Query(tc.sql, "public")
		} else {
			sc = Query(tc.sql)
		}

		if sc.Err() != tc.err {
			t.Errorf("expected no errors, but got %s", sc.Err())
		}

		if len(sc.Bytes()) < tc.jsonMinLen {
			t.Errorf("expected valid json response, but got %v", string(sc.Bytes()))
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
		sc := Query(tc.sql, "public")

		if sc.Err() == nil {
			t.Error("expected errors, but got nil")
		}

		if sc.Bytes() != nil {
			t.Errorf("expected no response, but got %s", string(sc.Bytes()))
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
		sql         string
		values      []interface{}
	}{
		{"Insert data into a table with one field", `INSERT INTO prest.public.test4(name) VALUES($1)`, []interface{}{"prest-test-insert"}},
		{"Insert data into a table with more than one field", `INSERT INTO prest.public.test5(name, celphone) VALUES($1, $2)`, []interface{}{"prest-test-insert", "88888888"}},
		{"Insert data into a table with more than one field and with quotes case sensitive", `INSERT INTO "prest"."public"."Reply"("name") VALUES($1)`, []interface{}{"prest-test-insert"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		sc := Insert(tc.sql, tc.values...)
		if sc.Err() != nil {
			t.Errorf("expected no errors, but got %s", sc.Err())
		}
		if len(sc.Bytes()) < 1 {
			t.Errorf("expected valid response body, but got %s", string(sc.Bytes()))
		}
	}
}

func TestInsertInvalid(t *testing.T) {
	var testCases = []struct {
		description string
		sql         string
		values      []interface{}
	}{
		{"Insert data into a table invalid database", "INSERT INTO 0prest.public.test4(name) VALUES($1)", []interface{}{"prest-test-insert"}},
		{"Insert data into a table invalid schema", "INSERT INTO prest.0public.test4(name) VALUES($1)", []interface{}{"prest-test-insert"}},
		{"Insert data into a table invalid table", "INSERT INTO prest.public.0test4(name) VALUES($1)", []interface{}{"prest-test-insert"}},
		{"Insert data into a table with empty name", "INSERT INTO (name) VALUES($1)", []interface{}{"prest-test-insert"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		sc := Insert(tc.sql, tc.values...)
		if sc.Err() == nil {
			t.Errorf("expected  errors, but no has")
		}
		if len(sc.Bytes()) > 0 {
			t.Errorf("expected valid response body, but got %s", string(sc.Bytes()))
		}
	}
}

func TestDelete(t *testing.T) {
	var testCases = []struct {
		description string
		sql         string
		values      []interface{}
	}{
		{"Try Delete data from invalid database", "DELETE FROM 0prest.public.test WHERE name=$1", []interface{}{"nuveo"}},
		{"Try Delete data from invalid schema", "DELETE FROM prest.0public.test WHERE name=$1", []interface{}{"nuveo"}},
		{"Try Delete data from invalid table", "DELETE FROM prest.public.0test WHERE name=$1", []interface{}{"nuveo"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		sc := Delete(tc.sql, tc.values)
		if sc.Err() == nil {
			t.Errorf("expected error, but got: %s", sc.Err())
		}

		if len(sc.Bytes()) > 0 {
			t.Errorf("expected empty response body, but got %s", string(sc.Bytes()))
		}
	}

	t.Log("Delete data from table")
	sc := Delete(`DELETE FROM "prest"."public"."test" WHERE "name"=$1`, "nuveo")
	if sc.Err() != nil {
		t.Errorf("expected no error, but got: %s", sc.Err())
	}

	if len(sc.Bytes()) < 1 {
		t.Errorf("expected response body, but got %s", string(sc.Bytes()))
	}
}

func TestUpdate(t *testing.T) {
	var testCases = []struct {
		description string
		sql         string
		values      []interface{}
	}{
		{"Update data into an invalid database", "UPDATE 0prest.publc.test3 SET name=$1", []interface{}{"prest tester"}},
		{"Update data into an invalid schema", "UPDATE prest.0publc.test3 SET name=$1", []interface{}{"prest tester"}},
		{"Update data into an invalid table", "UPDATE prest.publc.0test3 SET name=$1", []interface{}{"prest tester"}},
	}

	t.Log("Update data into a table")
	sc := Update(`UPDATE "prest"."public"."test" SET "name"=$2 WHERE "name"=$1`, "prest tester", "prest")
	if sc.Err() != nil {
		t.Errorf("expected no errors, but got: %s", sc.Err())
	}

	if len(sc.Bytes()) < 1 {
		t.Errorf("expected a valid response body, but got %s", string(sc.Bytes()))
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		sc := Update(tc.sql, tc.values...)
		if sc.Err() == nil {
			t.Errorf("expected error, but got: %s", sc.Err())
		}

		if len(sc.Bytes()) > 0 {
			t.Errorf("expected empty response body, but got %s", string(sc.Bytes()))
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
		{"SUM(test)", false},
		{`SUM("test")`, false},
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
		{"Join by request", "/prest/public/test?_join=inner:test2:test2.name:$eq:test.name", []string{"INNER JOIN", `"test2" ON `, `"test2"."name" = "test"."name"`}, false},
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
	var expectedSQL = []string{`"name" = $`, `"data"->>'description' = $`, " AND "}
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

	if !strings.Contains(joinStr, ` INNER JOIN "test2" ON "test2"."name" = "test"."name"`) {
		t.Errorf(`expected %s in INNER JOIN "test2" ON "test2"."name" = "test"."name", but no was!`, joinStr)
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
		{"Count fields from table", "/prest/public/test5?_count=celphone", `SELECT COUNT("celphone") FROM`, false},
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

		query, _ := DatabaseClause(r)
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

		query, _ := SchemaClause(r)
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
		{"$any", "ANY"},
		{"$some", "SOME"},
		{"$all", "ALL"},
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
	var expectedSQL = []string{"ORDER BY", `"name"`, `"number" DESC`}

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

	t.Log("Query ORDER BY with alias")
	expectedSQL = []string{"ORDER BY", `"c"."name"`, `"c"."number" DESC`}

	r, err = http.NewRequest("GET", "/prest/public/test?_order=c.name,-c.number", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, got: %v", err)
	}

	order, err = OrderByRequest(r)
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
		url         string
		table       string
		permission  string
		resultLen   int
	}{
		{"Read valid field", "/prest/public/test_list_only_id?_select=id", "test_list_only_id", "read", 1},
		{"Read invalid field", "/prest/public/test_list_only_id?_select=name", "test_list_only_id", "read", 0},
		{"Read non existing field", "/prest/public/test_list_only_id?_select=non_existing_field", "test_list_only_id", "read", 0},
		{"Select with *", "/prest/public/test_list_only_id?_select=*", "test_list_only_id", "read", 1},
		{"Select with group function", "/prest/public/test_group_by_table?_select=age,sum:salary&_groupby=age", "test_group_by_table", "read", 2},
	}

	for _, tc := range testCases {
		t.Log(tc.description)

		r, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, but got: %v", err)
		}

		fields, err := FieldsPermissions(r, tc.table, tc.permission)
		if err != nil {
			t.Errorf("expected no errors, but got %v", err)
		}
		if len(fields) != tc.resultLen {
			t.Errorf("expected %d in table: %s, got: %d - %v", tc.resultLen, tc.table, len(fields), fields)
		}
	}
}

func TestRestrictFalse(t *testing.T) {
	config.PrestConf.AccessConf.Restrict = false

	t.Log("Read unrestrict", config.PrestConf.AccessConf.Restrict)

	r, err := http.NewRequest("GET", "/prest/public/test_list_only_id?_select=*", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, but got: %v", err)
	}

	fields, err := FieldsPermissions(r, "test_list_only_id", "read")
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
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
		{"One field", []string{"test"}, `SELECT "test" FROM`},
		{"One field with alias", []string{"c.test"}, `SELECT "c"."test" FROM`},
		{"More field", []string{"test", "test02"}, `SELECT "test","test02" FROM`},
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
		{"Select array field from table", "/prest/public/testarray?_select=data", "data"},
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

type str struct{}

func (s str) String() string {
	return "test"
}

func TestFormatArray(t *testing.T) {
	testCases := []struct {
		name string
		in   interface{}
		ret  string
	}{
		{"array string", []string{"value 1", "value 2", "value 3"}, `{"value 1","value 2","value 3"}`},
		{"array int", []int{10, 20, 30}, `{10,20,30}`},
		{"empty array", []int{}, `{}`},
		{"stringer array", []fmt.Stringer{str{}, str{}, str{}}, `{"test","test","test"}`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ret := FormatArray(tc.in)
			if ret != tc.ret {
				t.Errorf("expected %v, but got %v", tc.ret, ret)
			}
		})
	}
}

func TestNormalizeGroupFunction(t *testing.T) {
	var testCases = []struct {
		description string
		urlValue    string
		expectedSQL string
	}{
		{"Normalize AVG Function", "avg:age", `AVG("age")`},
		{"Normalize SUM Function", "sum:age", `SUM("age")`},
		{"Normalize MAX Function", "max:age", `MAX("age")`},
		{"Normalize MIN Function", "min:age", `MIN("age")`},
		{"Normalize MEDIAN Function", "median:age", `MEDIAN("age")`},
		{"Normalize STDDEV Function", "stddev:age", `STDDEV("age")`},
		{"Normalize VARIANCE Function", "variance:age", `VARIANCE("age")`},
	}

	for _, tc := range testCases {
		partialSQL, err := NormalizeGroupFunction(tc.urlValue)
		if err != nil {
			t.Errorf("This function should not return error: %s", tc.description)
		}

		if tc.expectedSQL != partialSQL {
			t.Errorf("expected: %s, got: %s", tc.expectedSQL, partialSQL)
		}
	}
}

func TestCacheQuery(t *testing.T) {
	sc := Query(`SELECT * FROM "Reply"`)
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	sc = Query(`SELECT * FROM "Reply"`)
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
}

func TestCacheQueryCount(t *testing.T) {
	sc := QueryCount(`SELECT COUNT(*) FROM "Reply"`)
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	sc = QueryCount(`SELECT COUNT(*) FROM "Reply"`)
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
}

func TestCacheInsert(t *testing.T) {
	sc := Insert("INSERT INTO test(name) VALUES('testcache')")
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	sc = Insert("INSERT INTO test(name) VALUES('testcache')")
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
}

func TestCacheUpdate(t *testing.T) {
	sc := Update("UPDATE test SET name='test cache' WHERE name='testcache'")
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	sc = Update("UPDATE test SET name='test cache' WHERE name='testcache'")
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
}

func TestCacheDelete(t *testing.T) {
	sc := Delete("DELETE FROM test WHERE name='test cache'")
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	sc = Delete("DELETE FROM test WHERE name='test cache'")
	if err := sc.Err(); err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
}

func BenchmarkPrepare(b *testing.B) {
	db := connection.MustGet()
	for index := 0; index < b.N; index++ {
		_, err := Prepare(db, `SELECT * FROM "Reply"`)
		if err != nil {
			b.Fail()
		}
	}
}
