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
	. "github.com/smartystreets/goconvey/convey"
)

func TestWhereByRequest(t *testing.T) {
	Convey("Where by request without paginate", t, func() {
		r, err := http.NewRequest("GET", "/databases?dbname=eq.prest&test=eq.cool", nil)
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
		r, err := http.NewRequest("GET", "/prest/public/test?name=eq.nuveo&data->>description:jsonb=eq.bla", nil)
		So(err, ShouldBeNil)

		where, values, err := WhereByRequest(r, 1)
		So(err, ShouldBeNil)
		So(where, ShouldContainSubstring, "name=$")
		So(where, ShouldContainSubstring, "data->>'description'=$")
		So(where, ShouldContainSubstring, " AND ")
		So(values, ShouldContain, "nuveo")
		So(values, ShouldContain, "bla")
	})

	Convey("Where by request without jsonb key", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?name=eq.nuveo&data->>description:bla", nil)
		So(err, ShouldBeNil)

		_, _, err = WhereByRequest(r, 1)
		So(err, ShouldNotBeNil)
	})

	Convey("Where by request with jsonb field invalid", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?name=eq.nuveo&data->>0description:jsonb=eq.bla", nil)
		So(err, ShouldBeNil)

		_, _, err = WhereByRequest(r, 1)
		So(err, ShouldNotBeNil)
	})

	Convey("Where by request with field invalid", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?0name=eq.prest", nil)
		So(err, ShouldBeNil)

		_, _, err = WhereByRequest(r, 1)
		So(err, ShouldNotBeNil)
	})

	Convey("Where by request with invalid comparasion", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?name=pq.prest", nil)
		So(err, ShouldBeNil)

		_, _, err = WhereByRequest(r, 1)
		So(err, ShouldNotBeNil)
	})
}

func TestQuery(t *testing.T) {
	Convey("Query execution", t, func() {
		sql := "SELECT schema_name FROM information_schema.schemata ORDER BY schema_name ASC"
		json, err := Query(sql)
		So(err, ShouldBeNil)
		So(len(json), ShouldBeGreaterThan, 0)
	})

	Convey("Query execution 2", t, func() {
		sql := "SELECT number FROM prest.public.test2 ORDER BY number ASC"
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

	Convey("Query with invalid clause", t, func() {
		sql := "0SELECT schema_name FROM information_schema.schemata WHERE schema_name = $1 ORDER BY schema_name ASC"
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

	Convey("Paginate with invalid page value", t, func() {
		r, err := http.NewRequest("GET", "/databases?dbname=prest&test=cool&_page=X&_page_size=20", nil)
		So(err, ShouldBeNil)
		where, err := PaginateIfPossible(r)
		So(err, ShouldNotBeNil)
		So(where, ShouldContainSubstring, "")
	})

	Convey("Paginate with invalid page size value", t, func() {
		r, err := http.NewRequest("GET", "/databases?dbname=prest&test=cool&_page=1&_page_size=K", nil)
		So(err, ShouldBeNil)
		where, err := PaginateIfPossible(r)
		So(err, ShouldNotBeNil)
		So(where, ShouldContainSubstring, "")
	})

	Convey("Invalid Paginate if possible", t, func() {
		r, err := http.NewRequest("GET", "/databases?dbname=prest&test=cool", nil)
		So(err, ShouldBeNil)
		where, err := PaginateIfPossible(r)
		So(err, ShouldBeNil)
		So(where, ShouldContainSubstring, "")
	})
}

func TestInsert(t *testing.T) {
	config.InitConf()
	Convey("Insert data into a table with primary key", t, func() {
		m := make(map[string]interface{}, 0)

		m["name"] = "prest-test-insert"
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

	Convey("Insert data into a table with primary key named nuveo", t, func() {
		m := make(map[string]interface{}, 0)

		m["name"] = "prest-test-insert"
		r := api.Request{
			Data: m,
		}
		jsonByte, err := Insert("prest", "public", "test6", r)
		So(err, ShouldBeNil)
		So(len(jsonByte), ShouldBeGreaterThan, 0)

		var toJSON map[string]interface{}
		err = json.Unmarshal(jsonByte, &toJSON)
		So(err, ShouldBeNil)

		So(toJSON["nuveo"], ShouldEqual, 1)
	})

	Convey("Insert data into a table without primary key", t, func() {
		m := make(map[string]interface{}, 0)

		m["name"] = "prest-test-insert"
		r := api.Request{
			Data: m,
		}
		jsonByte, err := Insert("prest", "public", "test3", r)
		So(err, ShouldBeNil)
		So(len(jsonByte), ShouldBeGreaterThan, 0)

		var toJSON map[string]interface{}
		err = json.Unmarshal(jsonByte, &toJSON)
		So(err, ShouldBeNil)

		So(toJSON["name"], ShouldEqual, "prest-test-insert")
	})

	Convey("Insert invalid data into a table with primary key", t, func() {
		m := make(map[string]interface{}, 0)

		m["prest"] = "prest-test-insert"
		r := api.Request{
			Data: m,
		}
		jsonByte, err := Insert("prest", "public", "test6", r)
		So(err, ShouldNotBeNil)
		So(len(jsonByte), ShouldEqual, 0)

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

	Convey("Insert data into a database invalid", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Insert("0prest", "public", "test3", r)
		So(err, ShouldNotBeNil)
	})

	Convey("Insert data into a schema invalid", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Insert("prest", "0public", "test3", r)
		So(err, ShouldNotBeNil)
	})

	Convey("Insert data into a table invalid", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Insert("prest", "public", "0test3", r)
		So(err, ShouldNotBeNil)
	})

	Convey("Insert data into a request invalid", t, func() {
		m := make(map[string]interface{}, 0)
		m["0name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Insert("prest", "public", "test3", r)
		So(err, ShouldNotBeNil)
	})

	Convey("Insert data with more columns in table", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest-test-insert"
		m["celphone"] = "88888888888"
		r := api.Request{
			Data: m,
		}
		jsonByte, err := Insert("prest", "public", "test5", r)
		So(err, ShouldBeNil)
		So(len(jsonByte), ShouldBeGreaterThan, 0)
	})
}

func TestDelete(t *testing.T) {
	config.InitConf()
	Convey("Delete data from table", t, func() {
		json, err := Delete("prest", "public", "test", "name=$1", []interface{}{"nuveo"})
		So(err, ShouldBeNil)
		So(len(json), ShouldBeGreaterThan, 0)
	})

	Convey("Try Delete data from invalid database", t, func() {
		json, err := Delete("0prest", "public", "test", "name=$1", []interface{}{"nuveo"})
		So(err, ShouldNotBeNil)
		So(len(json), ShouldBeLessThanOrEqualTo, 0)
	})

	Convey("Try Delete data from invalid schema", t, func() {
		json, err := Delete("prest", "0public", "test", "name=$1", []interface{}{"nuveo"})
		So(err, ShouldNotBeNil)
		So(len(json), ShouldBeLessThanOrEqualTo, 0)
	})

	Convey("Try Delete data from invalid table", t, func() {
		json, err := Delete("prest", "0public", "test", "name=$1", []interface{}{"nuveo"})
		So(err, ShouldNotBeNil)
		So(len(json), ShouldBeLessThanOrEqualTo, 0)
	})
}

func TestUpdate(t *testing.T) {
	config.InitConf()
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

	Convey("Update data into a invalid database", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Update("0prest", "public", "test3", "name=$1", []interface{}{"prest tester"}, r)
		So(err, ShouldNotBeNil)
	})

	Convey("Update data into a invalid schema", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Update("prest", "0public", "test3", "name=$1", []interface{}{"prest tester"}, r)
		So(err, ShouldNotBeNil)
	})

	Convey("Update data into a invalid table", t, func() {
		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}
		_, err := Update("prest", "public", "0test3", "name=$1", []interface{}{"prest tester"}, r)
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

	Convey("Join empty param", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join", nil)
		So(err, ShouldBeNil)

		j, err := JoinByRequest(r)
		So(err, ShouldBeNil)
		So(j, ShouldBeNil)
	})

	Convey("Join missing param", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:test2:test2.name:$eq", nil)
		So(err, ShouldBeNil)

		_, err = JoinByRequest(r)
		So(err, ShouldNotBeNil)
	})
	Convey("Join invalid operator", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:test2:test2.name:notexist:test.name", nil)
		So(err, ShouldBeNil)

		_, err = JoinByRequest(r)
		So(err, ShouldNotBeNil)
	})

	Convey("Join invalid fields", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:0test2:test2.name:notexist:test.name", nil)
		So(err, ShouldBeNil)

		_, err = JoinByRequest(r)
		So(err, ShouldNotBeNil)
	})

	Convey("Join with where", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_join=inner:test2:test2.name:$eq:test.name&name=eq.nuveo&data->>description:jsonb=eq.bla", nil)
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

func TestCountFields(t *testing.T) {
	Convey("Count fields from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_count=celphone", nil)
		So(err, ShouldBeNil)

		countQuery, err := CountByRequest(r)
		So(countQuery, ShouldContainSubstring, "SELECT COUNT(celphone) FROM")
		So(err, ShouldBeNil)
	})

	Convey("Count all from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_count=*", nil)
		So(err, ShouldBeNil)

		countQuery, err := CountByRequest(r)
		So(countQuery, ShouldContainSubstring, "SELECT COUNT(*) FROM")
		So(err, ShouldBeNil)
	})

	Convey("Try Count with empty '_count' field", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_count=", nil)
		So(err, ShouldBeNil)

		countQuery, err := CountByRequest(r)
		So(countQuery, ShouldEqual, "")
		So(err, ShouldBeNil)
	})

	Convey("Try Count with invalid columns", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_count=celphone,0name", nil)
		So(err, ShouldBeNil)

		countQuery, err := CountByRequest(r)
		So(countQuery, ShouldEqual, "")
		So(err, ShouldNotBeNil)
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

	Convey("Query operator ne", t, func() {
		op, err := GetQueryOperator("$ne")
		So(err, ShouldBeNil)
		So(op, ShouldEqual, "!=")
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

	Convey("Query ORDER BY empty", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_order=", nil)
		So(err, ShouldBeNil)

		order, err := OrderByRequest(r)
		So(err, ShouldBeNil)
		So(order, ShouldContainSubstring, "")
	})

	Convey("Query ORDER BY invalid column", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test?_order=0name", nil)
		So(err, ShouldBeNil)

		order, err := OrderByRequest(r)
		So(err, ShouldNotBeNil)
		So(order, ShouldContainSubstring, "")
	})
}

func TestTablePermissions(t *testing.T) {
	config.InitConf()
	Convey("Read", t, func() {
		p := TablePermissions("test_readonly_access", "read")
		So(p, ShouldBeTrue)
	})
	Convey("Try to read without permission", t, func() {
		p := TablePermissions("test_write_and_delete_access", "read")
		So(p, ShouldBeFalse)
	})
	Convey("Write", t, func() {
		p := TablePermissions("test_write_and_delete_access", "write")
		So(p, ShouldBeTrue)
	})
	Convey("Try to write without permission", t, func() {
		p := TablePermissions("test_readonly_access", "write")
		So(p, ShouldBeFalse)
	})
	Convey("Delete", t, func() {
		p := TablePermissions("test_write_and_delete_access", "delete")
		So(p, ShouldBeTrue)
	})
	Convey("Try to delete without permission", t, func() {
		p := TablePermissions("test_readonly_access", "delete")
		So(p, ShouldBeFalse)
	})
	Convey("Restrict disabled", t, func() {
		config.PREST_CONF.AccessConf.Restrict = false
		p := TablePermissions("test_readonly_access", "delete")
		So(p, ShouldBeTrue)
	})

}

func TestFieldsPermissions(t *testing.T) {
	config.InitConf()

	Convey("Read valid field", t, func() {
		p := FieldsPermissions("test_list_only_id", []string{"id"}, "read")
		So(len(p), ShouldEqual, 1)
	})
	Convey("Read invalid field", t, func() {
		p := FieldsPermissions("test_list_only_id", []string{"name"}, "read")
		So(len(p), ShouldEqual, 0)
	})
	Convey("Read non existing field", t, func() {
		p := FieldsPermissions("test_list_only_id", []string{"non_existing_field"}, "read")
		So(len(p), ShouldEqual, 0)
	})
	Convey("Select with *", t, func() {
		p := FieldsPermissions("test_list_only_id", []string{"*"}, "read")
		So(len(p), ShouldEqual, 1)
	})
	Convey("Read unrestrict", t, func() {
		config.PREST_CONF.AccessConf.Restrict = false
		p := FieldsPermissions("test_list_only_id", []string{"*"}, "read")
		So(p[0], ShouldEqual, "*")
	})

}
func TestSelectFields(t *testing.T) {
	Convey("One field", t, func() {
		s, err := SelectFields([]string{"test"})
		So(s, ShouldContainSubstring, "SELECT test FROM")
		So(err, ShouldBeNil)
	})

	Convey("Two fields", t, func() {
		s, err := SelectFields([]string{"test", "test02"})
		So(s, ShouldContainSubstring, "test")
		So(s, ShouldContainSubstring, "test02")
		So(s, ShouldContainSubstring, "SELECT")
		So(s, ShouldContainSubstring, "FROM")
		So(err, ShouldBeNil)
	})

	Convey("Invalid fields", t, func() {
		_, err := SelectFields([]string{"0test", "test02"})
		So(err, ShouldNotBeNil)
	})

	Convey("Empty fields", t, func() {
		_, err := SelectFields([]string{})
		So(err, ShouldNotBeNil)
	})

}

func TestColumnsByRequest(t *testing.T) {
	Convey("Select fields from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=celphone", nil)
		So(err, ShouldBeNil)

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		So(selectStr, ShouldEqual, "celphone")
		So(len(selectQuery), ShouldEqual, 1)
	})
	Convey("Select all from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=*", nil)
		So(err, ShouldBeNil)

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		So(len(selectQuery), ShouldEqual, 1)
		So(selectStr, ShouldEqual, "*")
	})
	Convey("Try Select with empty '_select' field", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=", nil)
		So(err, ShouldBeNil)

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		So(len(selectQuery), ShouldEqual, 1)
		So(selectStr, ShouldEqual, "*")
	})
	Convey("Try Select with empty '_select' field", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=celphone,battery", nil)
		So(err, ShouldBeNil)

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		So(len(selectQuery), ShouldEqual, 2)
		So(selectStr, ShouldContainSubstring, "celphone,battery")
	})
}
