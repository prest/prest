package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/prest/prest/adapters/postgres/statements"
	"github.com/prest/prest/adapters/scanner"
	"github.com/prest/prest/config"
	"github.com/stretchr/testify/require"
	"github.com/structy/log"
)

func TestLoad(t *testing.T) {
	// Only run the failing part when a specific env variable is set
	if os.Getenv("BE_CRASHER") == "1" {
		t.Setenv("PREST_PG_DATABASE", "prest-test")
		return
	}
	// Start the actual test in a different subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestLoad")
	cmd.Env = append(os.Environ(), "BE_CRASHER=1")
	output, err := cmd.CombinedOutput()
	e, ok := err.(*exec.ExitError)
	if !ok || e.Success() {
		t.Fatalf("Process ran with err %v, want exit status 255", err)
	}
	log.Printf("%s\n %v\n", string(output), e.Error())
	if !cmd.ProcessState.Success() {
		os.Exit(0)
	}
}

func TestParseInsertRequest(t *testing.T) {
	// config.Load()
	// Load()
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
		require.NoError(t, err)

		req, err := http.NewRequest("POST", "/", bytes.NewReader(body))
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		adpt := NewAdapter(&config.Prest{})

		colsNames, _, values, err := adpt.ParseInsertRequest(req)
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
	mjl := make(map[string]interface{})
	mjl["prest"] = []string{"prest", "is", "awesome"}
	mjo := make(map[string]interface{})
	mjo["prest"] = "is marvelous"
	mjoJson, _ := json.Marshal(mjo)
	mjoStr := string(mjoJson)

	var testCases = []struct {
		description    string
		body           map[string]interface{}
		expectedSQL    []string
		expectedValues []string
		err            error
	}{
		{"set by request more than one field", mc, []string{`"dbname"=$`, `"test"=$`, ", "}, []string{"prest", "prest"}, nil},
		{"set by request one field", m, []string{`"name"=$`}, []string{"prest"}, nil},
		{"set by request one JSONB List field", mjl, []string{`"prest"=$`}, []string{`["prest", "is", "awesome"]`}, nil},
		{"set by request one JSONB Object field", mjo, []string{`"prest"=$`}, []string{mjoStr}, nil},
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

		adpt := NewAdapter(&config.Prest{})
		setSyntax, values, err := adpt.SetByRequest(req, 1)
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
		{"Where by request with spaced values", "/prest-test/public/test5?name=$eq.prest tester", []string{`"name" = $`}, []string{"prest tester"}, nil},
		{"Where by request with jsonb field", "/prest-test/public/test_jsonb_bug?name=$eq.goku&data->>description:jsonb=$eq.testing", []string{`"name" = $`, `"data"->>'description' = $`, " AND "}, []string{"goku", "testing"}, nil},
		{"Where by request with dot values", "/prest-test/public/test5?name=$eq.prest.txt tester", []string{`"name" = $`}, []string{"prest.txt tester"}, nil},
		{"Where by request with like", "/prest-test/public/test5?name=$like.%25val%25&phonenumber=123456", []string{`"name" LIKE $`, `"phonenumber" = $`, " AND "}, []string{"%val%", "123456"}, nil},
		{"Where by request with ilike", "/prest-test/public/test5?name=$ilike.%25vAl%25&phonenumber=123456", []string{`"name" ILIKE $`, `"phonenumber" = $`, " AND "}, []string{"%vAl%", "123456"}, nil},
		{"Where by request with not like", "/prest-test/public/test5?name=$nlike.%25val%25&phonenumber=123456", []string{`"name" NOT LIKE $`, `"phonenumber" = $`, " AND "}, []string{"%val%", "123456"}, nil},
		{"Where by request with not ilike", "/prest-test/public/test5?name=$nilike.%25vAl%25&phonenumber=123456", []string{`"name" NOT ILIKE $`, `"phonenumber" = $`, " AND "}, []string{"%vAl%", "123456"}, nil},
		{"Where by request with multiple colunm values", "/prest-test/public/table?created_at='$gte.1997-11-03'&created_at='$lte.1997-12-05'", []string{`"created_at" >= $`, ` AND `, `"created_at" <= $`}, []string{`'1997-11-03'`, `'1997-12-05'`}, nil},
		{"Where by request with tsquery", "/prest-test/public/test5?name:tsquery=prest", []string{`name @@ to_tsquery('prest')`}, []string{`'prest'`}, nil},
		{"Where by request with ltree left acendent", "/prest-test/public/test5?path='$ltreelanc.Top.*'", []string{`"path" @> $`}, []string{`'Top.*'`}, nil},
		{"Where by request with ltree right descendent", "/prest-test/public/test5?path='$ltreerdesc.Top.*'", []string{`"path" <@ $`}, []string{`'Top.*'`}, nil},
		{"Where by request with ltree match lquery", "/prest-test/public/test5?path='$ltreematch.Top.*'", []string{`"path" ~ $`}, []string{`'Top.*'`}, nil},
		{"Where by request with ltree match ltxtquery", "/prest-test/public/test5?path='$ltreematchtxt.Top*'", []string{`"path" @ $`}, []string{`'Top*'`}, nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		adpt := NewAdapter(&config.Prest{})
		where, values, err := adpt.WhereByRequest(req, 1)
		t.Log("where:", where)
		t.Log("values:", values)
		if err != nil {
			t.Errorf("expected no errors in where by request, got %v", err)
		}

		for _, sql := range tc.expectedSQL {
			if !strings.Contains(where, sql) {
				t.Errorf("expected %s in %s, but not was!", sql, where)
			}
		}

		expectedValuesSTR := strings.Join(tc.expectedValues, " ")
		t.Log("expectedValuesSTR:", expectedValuesSTR)
		for _, value := range values {
			t.Log("in values:", values)
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
		{"Where by request without jsonb key", "/prest-test/public/test_jsonb_bug?name=$eq.test&data->>description:bla"},
		{"Where by request with jsonb field invalid", "/prest-test/public/test_jsonb_bug?name=$eq.test&data->>0description:jsonb=$eq.bla"},
		{"Where by request with field invalid", "/prest-test/public/test?0name=$eq.prest"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		adpt := NewAdapter(&config.Prest{})
		where, values, err := adpt.WhereByRequest(req, 1)
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

func TestReturningByRequest(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		expectedSQL []string
		err         error
	}{
		{"Returning by request with nothing", "/prest-test/public/test_group_by_table", []string{""}, nil},
		{"Returning by request with _returning=*", "/prest-test/public/test_group_by_table?_returning=*", []string{"RETURNING *"}, nil},
		{"Returning by request with _returning=field", "/prest-test/public/test_group_by_table?_returning=age", []string{"RETURNING age"}, nil},
		{"Returning by request with multiple _returning=field", "/prest-test/public/test_group_by_table?_returning=age&_returning=salary", []string{"RETURNING age, salary"}, nil},
	}
	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}
		adpt := NewAdapter(&config.Prest{})
		returning, err := adpt.ReturningByRequest(req)
		t.Log("returning:", returning)
		if err != nil {
			t.Errorf("expected no errors in returning by request, got %v", err)
		}
		for _, sql := range tc.expectedSQL {
			if !strings.Contains(sql, returning) {
				t.Errorf("expected %s in %s, but not was!", sql, returning)
			}
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
		{"Group by clause with one field", "/prest-test/public/test5?_groupby=celphone", `GROUP BY "celphone"`, false},
		{"Group by clause with two fields", "/prest-test/public/test5?_groupby=celphone,name", `GROUP BY "celphone","name"`, false},
		{"Group by clause with two fields", "/prest-test/public/test5?_groupby=c.celphone,c.name", `GROUP BY "c"."celphone","c"."name"`, false},
		{"Group by clause without fields", "/prest-test/public/test5?_groupby=", "", true},

		// having tests
		{"Group by clause with having clause", "/prest-test/public/test5?_groupby=celphone->>having:sum:salary:$gt:500", `GROUP BY "celphone" HAVING SUM("salary") > 500`, false},
		{"Group by clause with having clause", "/prest-test/public/test5?_groupby=c.celphone->>having:sum:salary:$gt:500", `GROUP BY "c"."celphone" HAVING SUM("salary") > 500`, false},

		// having errors, but continue with group by
		{"Group by clause with wrong having clause (insufficient params)", "/prest-test/public/test5?_groupby=celphone->>having:sum:salary", `GROUP BY "celphone"`, false},
		{"Group by clause with wrong having clause (wrong query operator)", "/prest-test/public/test5?_groupby=celphone->>having:sum:salary:$at:500", `GROUP BY "celphone"`, false},
		{"Group by clause with wrong having clause (wrong group func)", "/prest-test/public/test5?_groupby=celphone->>having:sun:salary:$gt:500", `GROUP BY "celphone"`, false},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, got %v", err)
		}

		adpt := NewAdapter(&config.Prest{})
		groupBySQL := adpt.GroupByClause(req)

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

func TestQuery(t *testing.T) {
	var sc scanner.Scanner

	var testCases = []struct {
		description string
		sql         string
		param       bool
		jsonMinLen  int
		err         error
	}{
		{"Query execution", "SELECT schema_name FROM information_schema.schemata ORDER BY schema_name ASC", false, 1, nil},
		{"Query execution 2", `SELECT number FROM "prest-test"."public"."test2" ORDER BY number ASC`, false, 1, nil},
		{"Query execution with quotes", `SELECT "number" FROM "prest-test"."public"."test2" ORDER BY "number" ASC`, false, 1, nil},
		{"Query execution with params", "SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC", true, 1, nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		if tc.param {
			sc = adpt.Query(tc.sql, "public")
		} else {
			sc = adpt.Query(tc.sql)
		}
		if sc.Err() != tc.err {
			t.Errorf("expected no errors, but got %s", sc.Err())
		}

		if len(sc.Bytes()) < tc.jsonMinLen {
			t.Errorf("expected valid json response, but got %v", string(sc.Bytes()))
		}
	}
}

func TestQueryCtx(t *testing.T) {
	var sc scanner.Scanner

	ctx := context.Background()

	var testCases = []struct {
		description string
		sql         string
		param       bool
		jsonMinLen  int
		err         error
	}{
		{"Query execution", "SELECT schema_name FROM information_schema.schemata ORDER BY schema_name ASC", false, 1, nil},
		{"Query execution 2", `SELECT number FROM "prest-test"."public"."test2" ORDER BY number ASC`, false, 1, nil},
		{"Query execution with quotes", `SELECT "number" FROM "prest-test"."public"."test2" ORDER BY "number" ASC`, false, 1, nil},
		{"Query execution with params", "SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC", true, 1, nil},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		if tc.param {
			sc = adpt.QueryCtx(ctx, tc.sql, "public")
		} else {
			sc = adpt.QueryCtx(ctx, tc.sql)
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
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.Query(tc.sql, "public")

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
		{"Paginate if possible", "/databases?dbname=prest-test&test=cool&_page=1&_page_size=20", "LIMIT 20 OFFSET(1 - 1) * 20", nil},
		{"?_page negative", "/databases?dbname=prest-test&_page=0", "LIMIT 10 OFFSET(1 - 1) * 10", nil},
		{"Invalid Paginate if possible", "/databases?dbname=prest-test&test=cool", "", nil},
	}

	for _, tc := range testCase {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("url: %s / expected no errors in http request, but got %s", tc.url, err)
		}

		adpt := NewAdapter(&config.Prest{})
		sql, err := adpt.PaginateIfPossible(req)
		if err != nil {
			t.Errorf("desc: %s / expected no errors, but got %s", tc.description, err)
		}

		if !strings.Contains(tc.expected, sql) {
			t.Errorf("desc: %s / expected %s in %s, but not was!", tc.description, tc.expected, sql)
		}
	}
}

func TestInvalidPaginateIfPossible(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
	}{
		{"Paginate with invalid page value", "/databases?dbname=prest-test&test=cool&_page=X&_page_size=20"},
		{"Paginate with invalid page size value", "/databases?dbname=prest-test&test=cool&_page=1&_page_size=K"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, but got %s", err)
		}

		adpt := NewAdapter(&config.Prest{})
		sql, err := adpt.PaginateIfPossible(req)
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
		{"Insert data into a table with one field", `INSERT INTO "prest-test"."public"."test4"("name") VALUES($1)`, []interface{}{"prest-test-insert"}},
		{"Insert data into a table with more than one field", `INSERT INTO "prest-test"."public"."test5"("name", "celphone") VALUES($1, $2)`, []interface{}{"prest-test-insert", "88888888"}},
		{"Insert data into a table with more than one field and with quotes case sensitive", `INSERT INTO "prest-test"."public"."Reply"("name") VALUES($1)`, []interface{}{"prest-test-insert"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.Insert(tc.sql, tc.values...)
		if sc.Err() != nil {
			t.Errorf("expected no errors, but got %s", sc.Err())
		}
		if len(sc.Bytes()) < 1 {
			t.Errorf("expected valid response body, but got %s", string(sc.Bytes()))
		}
	}
}

func TestInsertCtx(t *testing.T) {
	ctx := context.Background()

	var testCases = []struct {
		description string
		sql         string
		values      []interface{}
	}{
		{"Insert data into a table with one field", `INSERT INTO "prest-test"."public"."test4"("name") VALUES($1)`, []interface{}{"prest-test-insert-ctx"}},
		{"Insert data into a table with more than one field", `INSERT INTO "prest-test"."public"."test5"("name", "celphone") VALUES($1, $2)`, []interface{}{"prest-test-insert-ctx", "88888888"}},
		{"Insert data into a table with more than one field and with quotes case sensitive", `INSERT INTO "prest-test"."public"."Reply"("name") VALUES($1)`, []interface{}{"prest-test-insert-ctx"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.InsertCtx(ctx, tc.sql, tc.values...)
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
		{"Insert data into a table invalid database", `INSERT INTO "0prest-test"."public"."test4"(name) VALUES($1)`, []interface{}{"prest-test-insert"}},
		{"Insert data into a table invalid schema", `INSERT INTO "prest-test"."0public"."test4"(name) VALUES($1)`, []interface{}{"prest-test-insert"}},
		{"Insert data into a table invalid table", `INSERT INTO "prest-test"."public-"."0test4"(name) VALUES($1)`, []interface{}{"prest-test-insert"}},
		{"Insert data into a table with empty name", `INSERT INTO (name) VALUES($1)`, []interface{}{"prest-test-insert"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.Insert(tc.sql, tc.values...)
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
		{"Try Delete data from invalid database", `DELETE FROM "0prest-test"."public"."test" WHERE name=$1`, []interface{}{"test"}},
		{"Try Delete data from invalid schema", `DELETE FROM "prest-test"."0public"."test" WHERE name=$1`, []interface{}{"test"}},
		{"Try Delete data from invalid table", `DELETE FROM "prest-test."public"."0test" WHERE name=$1`, []interface{}{"test"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.Delete(tc.sql, tc.values)
		if sc.Err() == nil {
			t.Errorf("expected error, but got: %s", sc.Err())
		}

		if len(sc.Bytes()) > 0 {
			t.Errorf("expected empty response body, but got %s", string(sc.Bytes()))
		}
	}

	t.Log("Delete data from table")
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.Delete(`DELETE FROM "prest-test"."public"."test" WHERE "name"=$1`, "test")
	if sc.Err() != nil {
		t.Errorf("expected no error, but got: %s", sc.Err())
	}

	if len(sc.Bytes()) < 1 {
		t.Errorf("expected response body, but got %s", string(sc.Bytes()))
	}
}

func TestDeleteCtx(t *testing.T) {
	ctx := context.Background()

	var testCases = []struct {
		description string
		sql         string
		values      []interface{}
	}{
		{"Try Delete data from invalid database", `DELETE FROM "0prest-test"."public"."test" WHERE name=$1`, []interface{}{"test"}},
		{"Try Delete data from invalid schema", `DELETE FROM "prest-test"."0public"."test" WHERE name=$1`, []interface{}{"test"}},
		{"Try Delete data from invalid table", `DELETE FROM "prest-test."public"."0test" WHERE name=$1`, []interface{}{"test"}},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.DeleteCtx(ctx, tc.sql, tc.values)
		if sc.Err() == nil {
			t.Errorf("expected error, but got: %s", sc.Err())
		}

		if len(sc.Bytes()) > 0 {
			t.Errorf("expected empty response body, but got %s", string(sc.Bytes()))
		}
	}

	t.Log("Delete data from table")
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.DeleteCtx(ctx, `DELETE FROM "prest-test"."public"."test" WHERE "name"=$1`, "test")
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
		{"Update data into an invalid database", `UPDATE "0prest-test"."public"."test3" SET name=$1`, []interface{}{"prest tester"}},
		{"Update data into an invalid schema", `UPDATE "prest-test"."0public"."test3" SET name=$1`, []interface{}{"prest tester"}},
		{"Update data into an invalid table", `UPDATE "prest-test"."public"."0test3" SET name=$1`, []interface{}{"prest tester"}},
	}

	t.Log("Update data into a table")
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.Update(`UPDATE "prest-test"."public"."test" SET "name"=$2 WHERE "name"=$1`, "prest tester", "prest")
	require.NoError(t, sc.Err())

	if len(sc.Bytes()) < 1 {
		t.Errorf("expected a valid response body, but got %s", string(sc.Bytes()))
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.Update(tc.sql, tc.values...)
		if sc.Err() == nil {
			t.Errorf("expected error, but got: %s", sc.Err())
		}

		if len(sc.Bytes()) > 0 {
			t.Errorf("expected empty response body, but got %s", string(sc.Bytes()))
		}
	}
}

func TestUpdateCtx(t *testing.T) {
	ctx := context.Background()

	var testCases = []struct {
		description string
		sql         string
		values      []interface{}
	}{
		{"Update data into an invalid database", `UPDATE "0prest-test"."public"."test3" SET name=$1`, []interface{}{"prest tester"}},
		{"Update data into an invalid schema", `UPDATE "prest-test"."0public"."test3" SET name=$1`, []interface{}{"prest tester"}},
		{"Update data into an invalid table", `UPDATE "prest-test"."public"."0test3" SET name=$1`, []interface{}{"prest tester"}},
	}

	t.Log("Update data into a table")
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.UpdateCtx(ctx, `UPDATE "prest-test"."public"."test" SET "name"=$2 WHERE "name"=$1`, "prest tester", "prest")
	if sc.Err() != nil {
		t.Errorf("expected no errors, but got: %s", sc.Err())
	}

	if len(sc.Bytes()) < 1 {
		t.Errorf("expected a valid response body, but got %s", string(sc.Bytes()))
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sc := adpt.UpdateCtx(ctx, tc.sql, tc.values...)
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
		{"not_invalid_table.with_not_invalid_column", false},
		{"not_long_enough_invalid_table_with_lots_of_chars.with_not_invalid_column", false},
		{"the_most_invalid_table_ever_seen_on_life_and_on_earth_at_least_today.the_most_invalid_column_ever_seen_on_life_and_on_earth_at_least_today", true},
		{`SUM("the_most_invalid_table_ever_seen_on_life_and_on_earth_at_least_today.the_most_invalid_column_ever_seen_on_life_and_on_earth_at_least_today")`, true},
		{`SUM("not_long_enough_invalid_table_with_lots_of_chars.with_not_invalid_column")`, false},
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
		{"Join by request", "/prest-test/public/test?_join=inner:test2:test2.name:$eq:test.name", []string{"INNER JOIN", `"test2" ON `, `"test2"."name" = "test"."name"`}, false},
		{"Join by request with schema", "/prest-test/public/test?_join=inner:public.test2:test2.name:$eq:test.name", []string{"INNER JOIN", `"public"."test2" ON `, `"test2"."name" = "test"."name"`}, false},
		{"Join empty params", "/prest-test/public/test?_join", []string{}, true},
		{"Join missing param", "/prest-test/public/test?_join=inner:test2:test2.name:$eq", []string{}, true},
		{"Join invalid operator", "/prest-test/public/test?_join=inner:test2:test2.name:notexist:test.name", []string{}, true},
		{"Join invalid fields", "/prest-test/public/test?_join=inner:0test2:test2.name:notexist:test.name", []string{}, true},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, got %v", err)
		}

		adpt := NewAdapter(&config.Prest{})
		join, err := adpt.JoinByRequest(req)
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
	var expectedValues = []string{"test", "bla"}

	r, err := http.NewRequest("GET", "/prest-test/public/test?_join=inner:test2:test2.name:$eq:test.name&name=$eq.test&data->>description:jsonb=$eq.bla", nil)
	if err != nil {
		t.Errorf("expected no errorn on New Request, got %v", err)
	}

	adpt := NewAdapter(&config.Prest{})
	join, err := adpt.JoinByRequest(r)
	if err != nil {
		t.Errorf("expected no errors, but got: %v", err)
	}

	joinStr := strings.Join(join, " ")

	if !strings.Contains(joinStr, ` INNER JOIN "test2" ON "test2"."name" = "test"."name"`) {
		t.Errorf(`expected %s in INNER JOIN "test2" ON "test2"."name" = "test"."name", but no was!`, joinStr)
	}

	where, values, err := adpt.WhereByRequest(r, 1)
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
		{"Count fields from table", "/prest-test/public/test5?_count=celphone",
			`SELECT COUNT("celphone") FROM`, false},
		{"Count all from table", "/prest-test/public/test5?_count=*", "SELECT COUNT(*) FROM", false},
		{"Count with empty params", "/prest-test/public/test5?_count=", "", false},
		{"Count with invalid columns", "/prest-test/public/test5?_count=celphone,0name", "", true},
		{"Count with `_groupby`", "/prest-test/public/test5?_count=celphone&_groupby=celphone",
			`SELECT COUNT("celphone") FROM`, false},
		{"Count with `_groupby` and `_select`", "/prest-test/public/test5?_count=celphone&_groupby=celphone&_select=celphone",
			`SELECT COUNT("celphone"), celphone FROM`, false},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, got: %v", err)
		}

		adpt := NewAdapter(&config.Prest{})
		sql, err := adpt.CountByRequest(req)
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

		adpt := NewAdapter(&config.Prest{})
		query, _ := adpt.DatabaseClause(r)
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

		adpt := NewAdapter(&config.Prest{})
		query, _ := adpt.SchemaClause(r)
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
		t.Logf("Query operator %s", tc.in)
		op, err := GetQueryOperator(tc.in)
		require.NoError(t, err)
		require.Equal(t, tc.out, op)
	}

	t.Log("Invalid query operator")
	op, err := GetQueryOperator("!lol")
	require.Error(t, err)
	require.Empty(t, op)
}

func TestOrderByRequest(t *testing.T) {
	t.Log("Query ORDER BY")
	var expectedSQL = []string{"ORDER BY", `"name"`, `"number" DESC`}

	r, err := http.NewRequest("GET", "/prest-test/public/test?_order=name,-number", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, got: %v", err)
	}

	adpt := NewAdapter(&config.Prest{})
	order, err := adpt.OrderByRequest(r)
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

	r, err = http.NewRequest("GET", "/prest-test/public/test?_order=c.name,-c.number", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, got: %v", err)
	}

	order, err = adpt.OrderByRequest(r)
	if err != nil {
		t.Errorf("expected no errors on OrderByRequest, got: %v", err)
	}
	for _, sql := range expectedSQL {
		if !strings.Contains(order, sql) {
			t.Errorf("expected %s in %s, but no was!", sql, order)
		}
	}

	t.Log("Query ORDER BY empty")
	r, err = http.NewRequest("GET", "/prest-test/public/test?_order=", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, got: %v", err)
	}

	order, err = adpt.OrderByRequest(r)
	if err != nil {
		t.Errorf("expected no errors on OrderByRequest, got: %v", err)
	}

	if order != "" {
		t.Errorf("expected order empty, got: %s", order)
	}

	t.Log("Query ORDER BY invalid column")
	r, err = http.NewRequest("GET", "/prest-test/public/test?_order=0name", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, got: %v", err)
	}

	order, err = adpt.OrderByRequest(r)
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
		{"Try config does not write", "test_permission_does_not_exist", "read", false},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		p := adpt.TablePermissions(tc.table, tc.permission)
		if p != tc.out {
			t.Errorf("expected %v, got %v", tc.out, p)
		}
	}

}

func TestSupportHyphenInTable(t *testing.T) {
	_, err := http.NewRequest("GET", "/prest-test/public/test-table-support-hyphen", nil)
	if err != nil {
		t.Errorf("expected no errors on Support Hyphen in Table, but got: %v", err)
	}
}

func TestRestrictFalse(t *testing.T) {
	// config.PrestConf.AccessConf.Restrict = false

	// t.Log("Read unrestrict", config.PrestConf.AccessConf.Restrict)

	r, err := http.NewRequest("GET", "/prest-test/public/test_list_only_id?_select=*", nil)
	if err != nil {
		t.Errorf("expected no errors on NewRequest, but got: %v", err)
	}

	adpt := NewAdapter(&config.Prest{})
	fields, err := adpt.FieldsPermissions(r, "test_list_only_id", "read")
	if err != nil {
		t.Errorf("expected no errors, but got %v", err)
	}
	if fields[0] != "*" {
		t.Errorf("expected '*', got: %s", fields[0])
	}

	t.Log("Restrict disabled")
	adpt = NewAdapter(&config.Prest{})
	p := adpt.TablePermissions("test_readonly_access", "delete")
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
		{"Aggregation Fields", []string{"max:age"}, `SELECT MAX("age") FROM`},
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
		adpt := NewAdapter(&config.Prest{})
		sql, err := adpt.SelectFields(tc.fields)
		if err != nil {
			t.Errorf("expected no errors, but got: %v", err)
		}

		if sql != tc.expectedSQL {
			t.Errorf("expected '%s', got: '%s'", tc.expectedSQL, sql)
		}
	}

	for _, tc := range testErrorCases {
		t.Log(tc.description)
		adpt := NewAdapter(&config.Prest{})
		sql, err := adpt.SelectFields(tc.fields)
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
		{"Select array field from table", "/prest-test/public/testarray?_select=data", "data"},
		{"Select fields from table", "/prest-test/public/test5?_select=celphone", "celphone"},
		{"Select all from table", "/prest-test/public/test5?_select=*", "*"},
		{"Select with empty '_select' field", "/prest-test/public/test5?_select=", ""},
		{"Select with more columns", "/prest-test/public/test5?_select=celphone,battery", "celphone,battery"},
		{"Select with more columns", "/prest-test/public/test5?_select=age,sum:salary&_groupby=age", `age,SUM("salary")`},
		{"Select with one aggregate function without group by", "/prest-test/public/test5?_select=sum:salary", `sum:salary`},
	}
	for _, tc := range testCases {
		r, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors on NewRequest, but got: %v", err)
		}

		selectQuery, _ := columnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		if selectStr != tc.expectedSQL {
			t.Errorf("expected %s, got: %s", tc.expectedSQL, selectStr)
		}
	}
}

func TestDistinctClause(t *testing.T) {
	var testCase = []struct {
		description string
		url         string
		expected    string
		err         error
	}{
		{"Valid distinct true", "/databases?dbname=prest-test&test=cool&_distinct=true", "SELECT DISTINCT", nil},
		{"Valid distinct false", "/databases?dbname=prest-test&test=cool&_distinct=false", "", nil},
		{"Invalid distinct", "/databases?dbname=prest-test&test=cool", "", nil},
	}

	for _, tc := range testCase {
		t.Log(tc.description)
		req, err := http.NewRequest("GET", tc.url, nil)
		if err != nil {
			t.Errorf("expected no errors in http request, but got %s", err)
		}

		adpt := NewAdapter(&config.Prest{})
		sql, err := adpt.DistinctClause(req)
		if err != nil {
			t.Errorf("expected no errors, but got %s", err)
		}

		if !strings.Contains(tc.expected, sql) {
			t.Errorf("expected %s in %s, but not was!", tc.expected, sql)
		}
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
		{"Normalize STDDEV Function", "stddev:age", `STDDEV("age")`},
		{"Normalize VARIANCE Function", "variance:age", `VARIANCE("age")`},
		{"Normalize AVG Function renamed", "avg:age:colname", `AVG("age") AS "colname"`},
		{"Normalize SUM Function renamed", "sum:age:colname", `SUM("age") AS "colname"`},
		{"Normalize MAX Function renamed", "max:age:colname", `MAX("age") AS "colname"`},
		{"Normalize MIN Function renamed", "min:age:colname", `MIN("age") AS "colname"`},
		{"Normalize STDDEV Function renamed", "stddev:age:colname", `STDDEV("age") AS "colname"`},
		{"Normalize VARIANCE Function renamed", "variance:age:colname", `VARIANCE("age") AS "colname"`},
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
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.Query(`SELECT * FROM "Reply"`)
	require.NoError(t, sc.Err())

	adpt = NewAdapter(&config.Prest{})
	sc = adpt.Query(`SELECT * FROM "Reply"`)
	require.NoError(t, sc.Err())
}

func TestCacheQueryCount(t *testing.T) {
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.QueryCount(`SELECT COUNT(*) FROM "Reply"`)
	require.NoError(t, sc.Err())

	adpt = NewAdapter(&config.Prest{})
	sc = adpt.QueryCount(`SELECT COUNT(*) FROM "Reply"`)
	require.NoError(t, sc.Err())
}

func TestCacheQueryCountCtx(t *testing.T) {
	adpt := NewAdapter(&config.Prest{})
	ctx := context.Background()
	sc := adpt.QueryCountCtx(ctx, `SELECT COUNT(*) FROM "Reply"`)
	require.NoError(t, sc.Err())

	adpt = NewAdapter(&config.Prest{})
	sc = adpt.QueryCountCtx(ctx, `SELECT COUNT(*) FROM "Reply"`)
	require.NoError(t, sc.Err())
}

func TestCacheInsert(t *testing.T) {
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.Insert("INSERT INTO test(name) VALUES('testcache')")
	require.NoError(t, sc.Err())

	adpt = NewAdapter(&config.Prest{})
	sc = adpt.Insert("INSERT INTO test(name) VALUES('testcache')")
	require.NoError(t, sc.Err())
}

func TestCacheUpdate(t *testing.T) {
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.Update("UPDATE test SET name='test cache' WHERE name='testcache'")
	require.NoError(t, sc.Err())

	adpt = NewAdapter(&config.Prest{})
	sc = adpt.Update("UPDATE test SET name='test cache' WHERE name='testcache'")
	require.NoError(t, sc.Err())
}

func TestCacheDelete(t *testing.T) {
	adpt := NewAdapter(&config.Prest{})
	sc := adpt.Delete("DELETE FROM test WHERE name='test cache'")
	require.NoError(t, sc.Err())

	adpt = NewAdapter(&config.Prest{})
	sc = adpt.Delete("DELETE FROM test WHERE name='test cache'")
	require.NoError(t, sc.Err())
}

// todo: fix these
// func BenchmarkPrepare(b *testing.B) {
// 	db := connection.MustGet()
// 	for index := 0; index < b.N; index++ {
// 		_, err := Prepare(db, `SELECT * FROM "Reply"`, false)
// 		if err != nil {
// 			b.Fail()
// 		}
// 	}
// }

func TestDisableCache(t *testing.T) {
	adpt := NewAdapter(&config.Prest{PGCache: false})

	adpt.ClearStmt()
	sc := adpt.Query(`SELECT * FROM "Reply"`)
	require.NoError(t, sc.Err())

	_, ok := adpt.stmt.PrepareMap[`SELECT jsonb_agg(s) FROM (SELECT * FROM "Reply") s`]
	require.False(t, ok, "has query in cache")
}

func TestParseBatchInsertRequest(t *testing.T) {
	m := make(map[string]interface{})
	m["name"] = "prest"
	m["pumpkin"] = "prest"
	records := make([]map[string]interface{}, 0)
	records = append(records, m)

	var testCases = []struct {
		description      string
		body             []map[string]interface{}
		expectedColNames string
		expectedValues   []interface{}
		err              error
	}{
		{
			"first test",
			records,
			`"name","pumpkin"`,
			[]interface{}{"prest", "prest"},
			nil,
		},
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

		adpt := NewAdapter(&config.Prest{})
		colsNames, _, values, err := adpt.ParseBatchInsertRequest(req)
		if err != tc.err {
			t.Errorf("expected errors %v in where by request, got %v", tc.err, err)
		}

		if tc.expectedColNames != colsNames {
			t.Errorf("expected %#v in %#v, but wasn't!", tc.expectedColNames, colsNames)
		}

		if !reflect.DeepEqual(tc.expectedValues, values) {
			t.Errorf("expected %v in %v", tc.expectedValues, values)
		}
	}
}

func TestBatchInsertValues(t *testing.T) {
	var testCases = []struct {
		description string
		sql         string
		records     []interface{}
	}{
		{
			"Insert data into a table with one field",
			`INSERT INTO "prest-test"."public"."test4"("name") VALUES($1),($2)`,
			[]interface{}{"1prest-test-batch-insert", "1batch-prest-test-insert"},
		}, {
			"Insert data into a table with more than one field",
			`INSERT INTO "prest-test"."public"."test5"("name", "celphone") VALUES($1, $2),($3, $4)`,
			[]interface{}{"2prest-test-batch-insert", "88888888", "2batch-prest-test-insert", "98888888"},
		}, {
			"Insert data into a table with more than one field and with quotes case sensitive",
			`INSERT INTO "prest-test"."public"."Reply"("name") VALUES($1),($2)`,
			[]interface{}{"3prest-test-batch-insert", "3batch-prest-test-insert"},
		},
	}

	for _, tc := range testCases {
		t.Log(tc.description)

		adpt := NewAdapter(&config.Prest{})
		sc := adpt.BatchInsertValues(tc.sql, tc.records...)
		require.NoError(t, sc.Err())

		if len(sc.Bytes()) < 2 {
			t.Errorf("expected valid response body, but got %s", string(sc.Bytes()))
		}
	}
}

func TestBatchInsertValuesCtx(t *testing.T) {

	var testCases = []struct {
		description string
		sql         string
		records     []interface{}
	}{
		{
			"Insert data into a table with one field",
			`INSERT INTO "prest-test"."public"."test4"("name") VALUES($1),($2)`,
			[]interface{}{"1prest-test-batch-insert-ctx", "1batch-prest-test-insert-ctx"},
		}, {
			"Insert data into a table with more than one field",
			`INSERT INTO "prest-test"."public"."test5"("name", "celphone") VALUES($1, $2),($3, $4)`,
			[]interface{}{"2prest-test-batch-insert", "88888888", "2batch-prest-test-insert", "98888888"},
		}, {
			"Insert data into a table with more than one field and with quotes case sensitive",
			`INSERT INTO "prest-test"."public"."Reply"("name") VALUES($1),($2)`,
			[]interface{}{"3prest-test-batch-insert-ctx", "3batch-prest-test-insert-ctx"},
		},
	}

	for _, tc := range testCases {
		t.Log(tc.description)

		adpt := NewAdapter(&config.Prest{})
		sc := adpt.BatchInsertValuesCtx(context.Background(), tc.sql, tc.records...)
		require.NoError(t, sc.Err())

		if len(sc.Bytes()) < 2 {
			t.Errorf("expected valid response body, but got %s", string(sc.Bytes()))
		}
	}
}

func TestPostgres_BatchInsertCopyCtx(t *testing.T) {
	ctx := context.Background()

	type args struct {
		dbname string
		schema string
		table  string
		keys   []string
		values []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"batch copy",
			args{
				"prest-test",
				"public",
				"Reply",
				[]string{`"name"`},
				[]interface{}{"copy-ctx"},
			},
			false,
		},
		{
			"batch copy without quotes",
			args{
				"prest-test",
				"public",
				"Reply",
				[]string{"name"},
				[]interface{}{"copy-ctx"},
			},
			false,
		},
		{
			"batch copy with err",
			args{
				"prest-test",
				"public",
				"Reply",
				[]string{"na"},
				[]interface{}{"copy-ctx"},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adpt := NewAdapter(&config.Prest{})

			sc := adpt.BatchInsertCopyCtx(ctx, tt.args.dbname, tt.args.schema, tt.args.table, tt.args.keys, tt.args.values...)
			require.Equal(t, tt.wantErr, sc.Err() != nil)
		})
	}
}

func TestPostgres_BatchInsertCopy(t *testing.T) {
	type args struct {
		dbname string
		schema string
		table  string
		keys   []string
		values []interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"batch copy",
			args{
				"prest-test",
				"public",
				"Reply",
				[]string{`"name"`},
				[]interface{}{"copy"},
			},
			false,
		},
		{
			"batch copy without quotes",
			args{
				"prest-test",
				"public",
				"Reply",
				[]string{"name"},
				[]interface{}{"copy"},
			},
			false,
		},
		{
			"batch copy with err",
			args{
				"prest-test",
				"public",
				"Reply",
				[]string{"na"},
				[]interface{}{"copy"},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adpt := NewAdapter(&config.Prest{})

			sc := adpt.BatchInsertCopy(tt.args.dbname, tt.args.schema, tt.args.table, tt.args.keys, tt.args.values...)
			require.Equal(t, tt.wantErr, sc.Err() != nil)
		})
	}
}

func TestPostgres_FieldsPermissions(t *testing.T) {
	type args struct {
		url    string
		table  string
		op     string
		fields []string
	}
	tests := []struct {
		name       string
		args       args
		restrict   bool
		wantFields []string
		wantErr    bool
	}{
		{
			name: "delete operations always returns *",
			args: args{
				op: "delete",
			},
			wantFields: []string{"*"},
		},
		{
			name:       "if restrict is false returns *",
			wantFields: []string{"*"},
		},
		{
			name: "error on parse groupby request",
			args: args{
				url: "/table_field_permission?_select=fail:fail&_groupby=fail",
			},
			restrict: true,
			wantErr:  true,
		},
		{
			name: "error with no allowed fields",
			args: args{
				url: "/table_field_permission",
			},
			restrict:   true,
			wantErr:    false,
			wantFields: []string{"*"},
		},
		{
			name: "allowed fields contains * and user don't pass select",
			args: args{
				url:    "/table_field_permission",
				table:  "test_field_permission",
				op:     "write",
				fields: []string{"*"},
			},
			restrict:   true,
			wantErr:    false,
			wantFields: []string{"*"},
		},
		{
			name: "allowed fields contains * and user ask for only only field",
			args: args{
				url:    "/table_field_permission?_select=name",
				table:  "test_field_permission",
				op:     "write",
				fields: []string{"*"},
			},
			restrict:   true,
			wantErr:    false,
			wantFields: []string{"name"},
		},
		{
			name: "allowed fields contains * and user ask for multiple fields",
			args: args{
				url:    "/table_field_permission?_select=name,age",
				table:  "test_field_permission",
				op:     "write",
				fields: []string{"*"},
			},
			restrict:   true,
			wantErr:    false,
			wantFields: []string{"name", "age"},
		},
		{
			name: "user ask for allowed field",
			args: args{
				url:    "/table_field_permission?_select=name",
				table:  "test_field_permission",
				op:     "write",
				fields: []string{"name", "age"},
			},
			restrict:   true,
			wantErr:    false,
			wantFields: []string{"name"},
		},
		{
			name: "user ask for not allowed field",
			args: args{
				url:    "/table_field_permission?_select=id",
				table:  "test_field_permission",
				op:     "write",
				fields: []string{"name", "age"},
			},
			restrict: true,
			wantErr:  false,
		},
		{
			name: "allowed some fields but user ask for nothing",
			args: args{
				url:    "/table_field_permission",
				table:  "test_field_permission",
				op:     "write",
				fields: []string{"name", "age"},
			},
			restrict:   true,
			wantErr:    false,
			wantFields: []string{"name", "age"},
		},
		{
			name: "functions in select should respect table permissions",
			args: args{
				url:    "/table_field_permission?_groupby=number&_select=max:number",
				table:  "test_field_permission",
				op:     "write",
				fields: []string{"name", "age"},
			},
			restrict: true,
			wantErr:  false,
		},
		{
			name: "select with function and allowed field returns field",
			args: args{
				url:    "/table_field_permission?_groupby=age&_select=max:age",
				table:  "test_field_permission",
				op:     "write",
				fields: []string{"name", "age"},
			},
			restrict:   true,
			wantErr:    false,
			wantFields: []string{`MAX("age")`},
		},
	}
	for _, tt := range tests {
		// config.PrestConf.AccessConf.Restrict = tt.restrict
		// config.PrestConf.AccessConf.Tables = []config.TablesConf{}
		// config.PrestConf.AccessConf.Tables = append(config.PrestConf.AccessConf.Tables,
		// 	config.TablesConf{
		// 		Name:        "test_field_permission",
		// 		Permissions: []string{"read", "write", "delete"},
		// 		Fields:      tt.args.fields,
		// 	})
		t.Run(tt.name, func(t *testing.T) {
			adapter := &adapter{}
			r, err := http.NewRequest(http.MethodGet, tt.args.url, strings.NewReader(""))
			if err != nil {
				t.Fatal(err)
			}
			gotFields, err := adapter.FieldsPermissions(r, tt.args.table, tt.args.op)
			if (err != nil) != tt.wantErr {
				t.Errorf("Postgres.FieldsPermissions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotFields, tt.wantFields) {
				t.Errorf("Postgres.FieldsPermissions() = %v, want %v", gotFields, tt.wantFields)
			}
		})
	}
}

func Test_intersection(t *testing.T) {
	type args struct {
		set   []string
		other []string
	}
	tests := []struct {
		name      string
		args      args
		wantInter []string
	}{
		{name: "two empty sets returns empty", wantInter: nil},
		{name: "intersection with empty set returns empty set", args: args{set: []string{"name"}, other: []string{}},
			wantInter: nil},
		{name: "intersection of empty set with other returns empty set", args: args{set: []string{}, other: []string{"name"}},
			wantInter: nil},
		{name: "intersection of two sets", args: args{set: []string{"name", "age"}, other: []string{"name"}},
			wantInter: []string{"name"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotInter := intersection(tt.args.set, tt.args.other); !reflect.DeepEqual(gotInter, tt.wantInter) {
				t.Errorf("intersection() = %v, want %v", gotInter, tt.wantInter)
			}
		})
	}
}

func Test_containsAsterisk(t *testing.T) {
	type args struct {
		arr []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "contains *",
			args: args{
				arr: []string{"*"},
			},
			want: true,
		},
		{
			name: "dont contains *",
			args: args{
				arr: []string{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsAsterisk(tt.args.arr); got != tt.want {
				t.Errorf("containsAsterisk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Postgres_GeneratesFuncs(t *testing.T) {

	// TEST FOR THESE FUNCTIONS:
	// SelectSQL
	// InsertSQL
	// DeleteSQL
	// UpdateSQL
	// DatabaseWhere
	// DatabaseOrderBy
	// SchemaOrderBy
	// TableClause --> THIS FUNCTION DONT NEED A TEST AT NOW...
	// TableWhere
	// TableOrderBy
	// SchemaTablesClause --> THIS FUNCTION DONT NEED A TEST AT NOW...
	// SchemaTablesWhere
	// SchemaTablesOrderBy

	//Postgres struct MOCK
	// will be used just to invoke the functions
	pg := adapter{}

	// all of this tests is for functions whats returns a string. We can use two string as args of tests. The string we will got by the function, and the expected string.
	type args struct {
		gotByFunc string
		expect    string
	}
	testCases := []struct {
		name string
		args args
	}{
		{
			name: "test_SelectSQL",
			args: args{
				gotByFunc: pg.SelectSQL("select", "database", "schema", "table"),
				expect:    `select "database"."schema"."table"`,
			},
		},
		{
			name: "test_InsertSQL",
			args: args{
				gotByFunc: pg.InsertSQL("database", "schema", "table", "names", "(name1, name2)"),
				expect:    `INSERT INTO "database"."schema"."table"(names) VALUES(name1, name2)`,
			},
		},
		{
			name: "test_DeleteSQL",
			args: args{
				gotByFunc: pg.DeleteSQL("database", "schema", "table"),
				expect:    `DELETE FROM "database"."schema"."table"`,
			},
		},
		{
			name: "test_UpdateSQL",
			args: args{
				gotByFunc: pg.UpdateSQL("database", "schema", "table", "syntax"),
				expect:    `UPDATE "database"."schema"."table" SET syntax`,
			},
		},
		{
			name: "test_DatabaseWhere_Empty",
			args: args{
				gotByFunc: pg.DatabaseWhere(""),
				expect:    statements.DatabasesWhere,
			},
		},
		{
			name: "test_DatabaseWhere_withRequest",
			args: args{
				gotByFunc: pg.DatabaseWhere("testrequestwhere"),
				expect:    fmt.Sprintf(`%v AND testrequestwhere`, statements.DatabasesWhere),
			},
		},
		{
			name: "test_DatabaseOrderBy",
			args: args{
				gotByFunc: pg.DatabaseOrderBy("order", true),
				expect:    "order",
			},
		},
		{
			name: "test_DatabaseOrderBy_Empty_hasCount",
			args: args{
				gotByFunc: pg.DatabaseOrderBy("", true),
				expect:    "",
			},
		},
		{
			name: "test_DatabaseOrderBy_Empty_nocount",
			args: args{
				gotByFunc: pg.DatabaseOrderBy("", false),
				expect: fmt.Sprintf(`
ORDER BY
	%s ASC`, statements.FieldDatabaseName),
			},
		},
		{
			name: "test_SchemaOrderBy",
			args: args{
				gotByFunc: pg.SchemaOrderBy("order", true),
				expect:    "order",
			},
		},
		{
			name: "test_SchemaOrderBy_Empty_hasCount",
			args: args{
				gotByFunc: pg.SchemaOrderBy("", true),
				expect:    "",
			},
		},
		{
			name: "test_SchemaOrderBy_Empty_nocount",
			args: args{
				gotByFunc: pg.SchemaOrderBy("", false),
				expect: fmt.Sprintf(`
ORDER BY
	%s ASC`, statements.FieldSchemaName),
			},
		},
		{
			name: "test_TableWhere",
			args: args{
				gotByFunc: pg.TableWhere("requestWhere"),
				expect:    fmt.Sprintf(`%v AND requestWhere`, statements.TablesWhere),
			},
		},
		{
			name: "test_TableWhere_Empty",
			args: args{
				gotByFunc: pg.TableWhere(""),
				expect:    statements.TablesWhere,
			},
		},
		{
			name: "test_TableOrderBy",
			args: args{
				gotByFunc: pg.TableOrderBy("order"),
				expect:    "order",
			},
		},
		{
			name: "test_TableOrderBy_Empty",
			args: args{
				gotByFunc: pg.TableOrderBy(""),
				expect:    statements.TablesOrderBy,
			},
		},
		{
			name: "test_SchemaTablesWhere",
			args: args{
				gotByFunc: pg.SchemaTablesWhere("requestWhere"),
				expect:    fmt.Sprintf("%v AND requestWhere", statements.SchemaTablesWhere),
			},
		},
		{
			name: "test_SchemaTablesWhere_Empty",
			args: args{
				gotByFunc: pg.SchemaTablesWhere(""),
				expect:    statements.SchemaTablesWhere,
			},
		},
		{
			name: "test_SchemaTablesOrderBy",
			args: args{
				gotByFunc: pg.SchemaTablesOrderBy("order"),
				expect:    "order",
			},
		},
		{
			name: "test_SchemaTablesOrderBy_Empty",
			args: args{
				gotByFunc: pg.SchemaTablesOrderBy(""),
				expect:    statements.SchemaTablesOrderBy,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.args.gotByFunc != tc.args.expect {
				t.Errorf("Got: %v, Want: %v", tc.args.gotByFunc, tc.args.expect)
			}
		})
	}

}

func Test_ShowTable(t *testing.T) {
	pg := adapter{}
	sc := pg.ShowTable("testschema", "testtable")

	scMock := newScannerMock(t)

	if !reflect.DeepEqual(sc, scMock) {
		t.Errorf("Should return a adpter Scanner with this structure: {[] <nil> true}; but got: %v", sc)
	}
}

func Test_ShowTableCtx(t *testing.T) {
	pg := adapter{}

	sc := pg.ShowTableCtx(context.Background(), "testschema", "testtable")

	scMock := newScannerMock(t)
	if !reflect.DeepEqual(sc, scMock) {
		t.Errorf("Should return a adpter Scanner with this structure: {[] <nil> true}; but got: %v", sc)
	}
}

func newScannerMock(t *testing.T) (sc scanner.Scanner) {
	t.Helper()
	return &scanner.PrestScanner{
		Buff:    bytes.NewBuffer([]byte("[]")),
		Error:   nil,
		IsQuery: true,
	}
}

func TestSliceToJSONList(t *testing.T) {
	var i interface{}
	var testCases = []struct {
		description   string
		body          interface{}
		expectedValue string
		err           error
	}{
		{"String slice", []string{"one", "two", "three"}, `["one", "two", "three"]`, nil},
		{"Integer slice", []int{1, 2, 3}, `[1, 2, 3]`, nil},
		{"Interface error", i, `[]`, ErrEmptyOrInvalidSlice},
	}

	for _, tc := range testCases {
		t.Log(tc.description)

		value, err := sliceToJSONList(tc.body)

		if !strings.Contains(tc.expectedValue, value) {
			t.Errorf("expected %s in %s", value, tc.expectedValue)
		}

		if err != tc.err {
			t.Errorf("expected %s in %s", err, tc.err)
		}
	}
}

func Test_NewAdapter(t *testing.T) {
	cfg := &config.Prest{
		PGDatabase: "prest",
		// Add other necessary fields in the config struct
	}

	adapter := NewAdapter(cfg)

	// Verify that the adapter is created correctly
	require.Equal(t, adapter.cfg, cfg)

	// Verify that the stmt field is initialized correctly
	require.NotNil(t, adapter.stmt)
	require.NotNil(t, adapter.stmt.PrepareMap)

	// Verify that the conn field is initialized correctly
	require.NotNil(t, adapter.pool)
}
