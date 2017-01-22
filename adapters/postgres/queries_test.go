package postgres

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/user"
	"testing"

	"github.com/nuveo/prest/config"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMain(m *testing.M) {
	config.InitConf()
	createMockScripts(config.PREST_CONF.QueriesPath)
	writeMockScripts(config.PREST_CONF.QueriesPath)

	code := m.Run()

	removeMockScripts(config.PREST_CONF.QueriesPath)
	os.Exit(code)
}

func TestGetScript(t *testing.T) {
	Convey("Get script file by GET method", t, func() {
		_, err := GetScript("GET", "fulltable", "get_all")
		So(err, ShouldBeNil)
	})

	Convey("Get script file by POST method", t, func() {
		_, err := GetScript("POST", "fulltable", "write_all")
		So(err, ShouldBeNil)
	})

	Convey("Get script file by PATCH method", t, func() {
		_, err := GetScript("PATCH", "fulltable", "patch_all")
		So(err, ShouldBeNil)
	})

	Convey("Get script file by PUT method", t, func() {
		_, err := GetScript("PUT", "fulltable", "put_all")
		So(err, ShouldBeNil)
	})

	Convey("Get script file by DELETE method", t, func() {
		_, err := GetScript("DELETE", "fulltable", "delete_all")
		So(err, ShouldBeNil)
	})

	Convey("Get script file by invalid method", t, func() {
		_, err := GetScript("ANY", "fulltable", "delete_all")
		So(err, ShouldNotBeNil)
	})

	Convey("Try get script nonexistent", t, func() {
		_, err := GetScript("GET", "fulltable", "ooooelete_all")
		So(err, ShouldNotBeNil)
	})

	Convey("Try get script with nonexistent folder", t, func() {
		_, err := GetScript("GET", "fue", "get_all")
		So(err, ShouldNotBeNil)
	})
}

func TestParseScript(t *testing.T) {
	queryURL := url.Values{}
	queryURL.Set("field1", "abc")

	user, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	scriptPath := fmt.Sprint(user.HomeDir, "/queries/fulltable/%s")
	Convey("Parse Script with get_all file", t, func() {
		sql, values, err := ParseScript(fmt.Sprintf(scriptPath, "get_all.read.sql"), queryURL)
		So(err, ShouldBeNil)
		So("abc", ShouldBeIn, values)
		So(sql, ShouldEqual, "SELECT * FROM test7 WHERE name = $1")
	})

	queryURL.Del("field1")

	// Add new values
	queryURL.Set("notable", "123")

	Convey("Try Parse Script with invalid params", t, func() {
		sql, values, err := ParseScript(fmt.Sprintf(scriptPath, "get_all.read.sql"), queryURL)
		So(err, ShouldNotBeNil)
		So("123", ShouldBeIn, values)
		So(sql, ShouldEqual, "")
	})
}

func TestCreateSQL(t *testing.T) {

	Convey("Execute a valid INSERT sql", t, func() {
		sql := "INSERT INTO test7 (name) values ($1) RETURNING id"
		values := []interface{}{"lulu"}

		result, err := CreateSQL(sql, values)
		So(err, ShouldBeNil)
		So(len(result), ShouldBeGreaterThan, 0)
	})

	Convey("Execute an invalid INSERT sql without RETURNING clause", t, func() {
		sql := "INSERT INTO test7 (name) values ($1)"
		values := []interface{}{"lulu"}

		result, err := CreateSQL(sql, values)
		So(err, ShouldNotBeNil)
		So(len(result), ShouldEqual, 0)
	})

	Convey("Execute an invalid INSERT sql", t, func() {
		sql := "INSERT INTO test7 (tool) values ($1) RETURNING id"
		values := []interface{}{"lulu"}

		result, err := CreateSQL(sql, values)
		So(err, ShouldNotBeNil)
		So(len(result), ShouldEqual, 0)
	})
}

func TestWriteSQL(t *testing.T) {

	Convey("Execute a valid UPDATE sql", t, func() {
		sql := "UPDATE test7 SET name = $1 WHERE surname = $2"
		values := []interface{}{"lulu", "temer"}

		result, err := WriteSQL(sql, values)
		So(err, ShouldBeNil)
		So(len(result), ShouldBeGreaterThan, 0)
	})

	Convey("Execute a valid DELETE sql", t, func() {
		sql := "DELETE FROM test7 WHERE name = $1"
		values := []interface{}{"lulu"}

		result, err := WriteSQL(sql, values)
		So(err, ShouldBeNil)
		So(len(result), ShouldBeGreaterThan, 0)
	})

	Convey("Execute an invalid UPDATE sql", t, func() {
		sql := "UPDATE test7 SET name = $1 WHERE surname = $2"
		values := []interface{}{"lulu"}

		result, err := WriteSQL(sql, values)
		So(err, ShouldNotBeNil)
		So(len(result), ShouldEqual, 0)
	})

	Convey("Execute an invalid DELETE sql", t, func() {
		sql := "DELETE FROM test7 WHERE name = $1 AND surname = $2"
		values := []interface{}{"lulu"}

		result, err := WriteSQL(sql, values)
		So(err, ShouldNotBeNil)
		So(len(result), ShouldEqual, 0)
	})
}

func TestExecuteScripts(t *testing.T) {
	Convey("Get errors with invalid HTTP method", t, func() {
		values := make([]interface{}, 0)
		result, err := ExecuteScripts("ANY", "SELECT * FROM test7", values)
		So(len(result), ShouldEqual, 0)
		So(err, ShouldNotBeNil)
	})

	Convey("Get result with GET HTTP method", t, func() {
		values := make([]interface{}, 0)
		result, err := ExecuteScripts("GET", "SELECT * FROM test7", values)
		So(len(result), ShouldBeGreaterThan, 0)
		So(err, ShouldBeNil)
	})

	Convey("Get result with POST HTTP method", t, func() {
		values := []interface{}{"lala"}
		result, err := ExecuteScripts("POST", "INSERT INTO test7 (name) VALUES ($1) RETURNING id", values)
		So(len(result), ShouldBeGreaterThan, 0)
		So(err, ShouldBeNil)
	})

	Convey("Get result with PUT HTTP method", t, func() {
		values := []interface{}{"lala", "temer"}
		result, err := ExecuteScripts("PUT", "UPDATE test7 SET name = $1 WHERE surname = $2", values)
		So(len(result), ShouldBeGreaterThan, 0)
		So(err, ShouldBeNil)
	})

	Convey("Get result with PATCH HTTP method", t, func() {
		values := []interface{}{"temer", "lala"}
		result, err := ExecuteScripts("PATCH", "UPDATE test7 SET surname = $1 WHERE name = $2", values)
		So(len(result), ShouldBeGreaterThan, 0)
		So(err, ShouldBeNil)
	})

	Convey("Get result with DELETE HTTP method", t, func() {
		values := []interface{}{"lala"}
		result, err := ExecuteScripts("DELETE", "DELETE FROM test7 WHERE surname = $1", values)
		So(len(result), ShouldBeGreaterThan, 0)
		So(err, ShouldBeNil)
	})
}
