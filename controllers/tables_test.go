package controllers

import (
	"net/http"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetTables(t *testing.T) {
	Convey("Get tables without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/tables", nil)
		So(err, ShouldBeNil)
		validate(r, GetTables)
	})

	Convey("Get tables with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/tables?c.relname=prest", nil)
		So(err, ShouldBeNil)
		validate(r, GetTables)
	})
}

func TestGetTablesByDatabaseAndSchema(t *testing.T) {
	Convey("Get tables by database and schema without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/prest/public", nil)
		So(err, ShouldBeNil)
		validate(r, GetTablesByDatabaseAndSchema)
	})

	Convey("Get tables by database and schema with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/prest/public?t.tablename=test", nil)
		So(err, ShouldBeNil)
		validate(r, GetTablesByDatabaseAndSchema)
	})
}
