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
		r, err := http.NewRequest("GET", "/tables?name=prest", nil)
		So(err, ShouldBeNil)
		validate(r, GetTables)
	})
}
