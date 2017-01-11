package postgres

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/statements"
	. "github.com/smartystreets/goconvey/convey"
)

func TestWhereByRequest(t *testing.T) {
	Convey("Where by request without paginate", t, func() {
		r, err := http.NewRequest("GET", "/databases?dbname=prest&test=cool", nil)
		So(err, ShouldBeNil)

		where, values, err := WhereByRequest(r, 1)
		So(err, ShouldBeNil)
		So(where, ShouldContainSubstring, "dbname=$")
		So(where, ShouldContainSubstring, "test=$")
		So(where, ShouldContainSubstring, " AND ")
		So(values, ShouldContain, "prest")
		So(values, ShouldContain, "cool")
	})

	Convey("Where by request with jsonb field", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?name=nuveo&data->>description:jsonb=bla", nil)
		So(err, ShouldBeNil)

		where, values, err := WhereByRequest(r, 1)
		So(err, ShouldBeNil)
		So(where, ShouldContainSubstring, "name=$")
		So(where, ShouldContainSubstring, "data->>'description'=$")
		So(where, ShouldContainSubstring, " AND ")
		So(values, ShouldContain, "nuveo")
		So(values, ShouldContain, "bla")
	})
}

func TestQuery(t *testing.T) {
	Convey("Query execution", t, func() {
		sql := "SELECT schema_name FROM information_schema.schemata ORDER BY schema_name ASC"
		json, err := Query(sql)
		So(err, ShouldBeNil)
		So(len(json), ShouldBeGreaterThan, 0)
	})

	Convey("Query execution with params", t, func() {
		sql := "SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC"
		json, err := Query(sql, "public")
		So(err, ShouldBeNil)
		So(len(json), ShouldBeGreaterThan, 0)
	})

	Convey("Query with invalid characters", t, func() {
		sql := "SELECT ~~, ``, ˜ schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC"
		json, err := Query(sql, "public")
		So(err, ShouldNotBeNil)
		So(json, ShouldBeNil)
	})

}

func TestPaginateIfPossible(t *testing.T) {
	Convey("Paginate if possible", t, func() {
		r, err := http.NewRequest("GET", "/databases?dbname=prest&test=cool&_page=1&_page_size=20", nil)
		So(err, ShouldBeNil)
		where, err := PaginateIfPossible(r)
		So(err, ShouldBeNil)
		So(where, ShouldContainSubstring, "LIMIT 20 OFFSET(1 - 1) * 20")
	})
}

func TestInsert(t *testing.T) {
	Convey("Insert data into a table", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		jsonByte, err := Insert("prest", "public", "test4", r)
		So(err, ShouldBeNil)
		So(len(jsonByte), ShouldBeGreaterThan, 0)

		var toJSON map[string]interface{}
		err = json.Unmarshal(jsonByte, &toJSON)
		So(err, ShouldBeNil)

		So(toJSON["id"], ShouldEqual, 1)
	})

	Convey("Insert data into a table with contraints", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Insert("prest", "public", "test3", r)
		So(err, ShouldNotBeNil)
	})
}

func TestDelete(t *testing.T) {
	Convey("Delete data from table", t, func() {
		json, err := Delete("prest", "public", "test", "name=$1", []interface{}{"nuveo"})
		So(err, ShouldBeNil)
		So(len(json), ShouldBeGreaterThan, 0)
	})
}

func TestUpdate(t *testing.T) {
	Convey("Update data into a table", t, func() {

		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		json, err := Update("prest", "public", "test", "name=$1", []interface{}{"prest"}, r)
		So(err, ShouldBeNil)
		So(len(json), ShouldBeGreaterThan, 0)
	})

	Convey("Update data into a table with constraints", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Update("prest", "public", "test3", "name=$1", []interface{}{"prest tester"}, r)
		So(err, ShouldNotBeNil)
	})
}

func TestChkInvaidIdentifier(t *testing.T) {
	Convey("Check invalid character on identifier", t, func() {
		chk := chkInvalidIdentifier("fildName")
		So(chk, ShouldBeFalse)
		chk = chkInvalidIdentifier("_9fildName")
		So(chk, ShouldBeFalse)
		chk = chkInvalidIdentifier("_fild.Name")
		So(chk, ShouldBeFalse)

		chk = chkInvalidIdentifier("0fildName")
		So(chk, ShouldBeTrue)
		chk = chkInvalidIdentifier("fild'Name")
		So(chk, ShouldBeTrue)
		chk = chkInvalidIdentifier("fild\"Name")
		So(chk, ShouldBeTrue)
		chk = chkInvalidIdentifier("fild;Name")
		So(chk, ShouldBeTrue)
		chk = chkInvalidIdentifier("_123456789_123456789_123456789_123456789_123456789_123456789_12345")
		So(chk, ShouldBeTrue)

	})
}

func TestJoinByRequest(t *testing.T) {
	Convey("Join by request", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:test2:test2.name:$eq:test.name", nil)
		So(err, ShouldBeNil)

		join, err := JoinByRequest(r)
		joinStr := strings.Join(join, " ")

		So(err, ShouldBeNil)
		So(joinStr, ShouldContainSubstring, "INNER JOIN test2 ON test2.name = test.name")
	})
	Convey("Join missing param", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:test2:test2.name:$eq", nil)
		So(err, ShouldNotBeNil)

		_, err = JoinByRequest(r)
		So(err, ShouldNotBeNil)
	})
	Convey("Join invalid operator", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:test2:test2.name:notexist:test.name", nil)
		So(err, ShouldBeNil)

		_, err = JoinByRequest(r)
		So(err, ShouldNotBeNil)
	})
	Convey("Join with where", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:test2:test2.name:$eq:test.name&name=nuveo&data->>description:jsonb=bla", nil)
		So(err, ShouldBeNil)

		join, err := JoinByRequest(r)
		joinStr := strings.Join(join, " ")

		So(err, ShouldBeNil)
		So(joinStr, ShouldContainSubstring, "INNER JOIN test2 ON test2.name = test.name")

		where, values, err := WhereByRequest(r, 1)
		So(err, ShouldBeNil)
		So(where, ShouldContainSubstring, "name=$")
		So(where, ShouldContainSubstring, "data->>'description'=$")
		So(where, ShouldContainSubstring, " AND ")
		So(values, ShouldContain, "nuveo")
		So(values, ShouldContain, "bla")
	})

}

func TestSelectFields(t *testing.T) {
	Convey("Select fields from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=celphone", nil)
		So(err, ShouldBeNil)

		selectQuery := SelectByRequest(r)
		So(selectQuery, ShouldContainSubstring, "SELECT celphone FROM")
	})

	Convey("Select all from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=*", nil)
		So(err, ShouldBeNil)

		selectQuery := SelectByRequest(r)
		So(selectQuery, ShouldContainSubstring, "SELECT * FROM")
	})

	Convey("Try Select with empty '_select' field", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=", nil)
		So(err, ShouldBeNil)

		selectQuery := SelectByRequest(r)
		So(selectQuery, ShouldEqual, "")
	})
}

func TestCountFields(t *testing.T) {
	Convey("Count fields from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_count=celphone", nil)
		So(err, ShouldBeNil)

		countQuery := CountByRequest(r)
		So(countQuery, ShouldContainSubstring, "SELECT COUNT(celphone) FROM")
	})

	Convey("Count all from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_count=*", nil)
		So(err, ShouldBeNil)

		countQuery := CountByRequest(r)
		So(countQuery, ShouldContainSubstring, "SELECT COUNT(*) FROM")
	})

	Convey("Try Count with empty '_count' field", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_count=", nil)
		So(err, ShouldBeNil)

		countQuery := CountByRequest(r)
		So(countQuery, ShouldEqual, "")
	})
}

func TestDatabaseClause(t *testing.T) {
	Convey("Return appropriate SELECT clause", t, func() {
		r, err := http.NewRequest("GET", "/databases", nil)
		So(err, ShouldBeNil)

		countQuery := DatabaseClause(r)
		So(countQuery, ShouldEqual, fmt.Sprintf(statements.DatabasesSelect, statements.FieldDatabaseName))
	})

	Convey("Return appropriate COUNT clause", t, func() {
		r, err := http.NewRequest("GET", "/databases?_count=*", nil)
		So(err, ShouldBeNil)

		countQuery := DatabaseClause(r)
		So(countQuery, ShouldEqual, fmt.Sprintf(statements.DatabasesSelect, statements.FieldCountDatabaseName))
	})
}

func TestSchemaClause(t *testing.T) {
	Convey("Return appropriate SELECT clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas", nil)
		So(err, ShouldBeNil)

		countQuery := SchemaClause(r)
		So(countQuery, ShouldEqual, fmt.Sprintf(statements.SchemasSelect, statements.FieldSchemaName))
	})

	Convey("Return appropriate COUNT clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas?_count=*", nil)
		So(err, ShouldBeNil)

		countQuery := SchemaClause(r)
		So(countQuery, ShouldEqual, fmt.Sprintf(statements.SchemasSelect, statements.FieldCountSchemaName))
	})
}

func TestGetQueryOperator(t *testing.T) {
	Convey("Query operator eq", t, func() {
		op, err := GetQueryOperator("$eq")
		So(err, ShouldBeNil)
		So(op, ShouldEqual, "=")
	})
	Convey("Query operator gt", t, func() {
		op, err := GetQueryOperator("$gt")
		So(err, ShouldBeNil)
		So(op, ShouldEqual, ">")
	})
	Convey("Query operator gte", t, func() {
		op, err := GetQueryOperator("$gte")
		So(err, ShouldBeNil)
		So(op, ShouldEqual, ">=")
	})

	Convey("Query operator lt", t, func() {
		op, err := GetQueryOperator("$lt")
		So(err, ShouldBeNil)
		So(op, ShouldEqual, "<")
	})
	Convey("Query operator lte", t, func() {
		op, err := GetQueryOperator("$lte")
		So(err, ShouldBeNil)
		So(op, ShouldEqual, "<=")
	})
	Convey("Query operator IN", t, func() {
		op, err := GetQueryOperator("$in")
		So(err, ShouldBeNil)
		So(op, ShouldEqual, "IN")
	})
	Convey("Query operator NIN", t, func() {
		op, err := GetQueryOperator("$nin")
		So(err, ShouldBeNil)
		So(op, ShouldEqual, "NOT IN")
	})
}

func TestOrderByRequest(t *testing.T) {
	Convey("Query ORDER BY", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_order=name,-number", nil)
		So(err, ShouldBeNil)

		order, err := OrderByRequest(r)
		So(err, ShouldBeNil)
		So(order, ShouldContainSubstring, "ORDER BY")
		So(order, ShouldContainSubstring, "name")
		So(order, ShouldContainSubstring, "number DESC")
	})
}
