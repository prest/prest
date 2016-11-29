package controllers

import (
	"net/http"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetSchemas(t *testing.T) {
	Convey("Get schemas without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas", nil)
		So(err, ShouldBeNil)
		validate(r, GetSchemas)
	})

	Convey("Get schemas with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas?schema_name=public", nil)
		So(err, ShouldBeNil)
		validate(r, GetSchemas)
	})
}
