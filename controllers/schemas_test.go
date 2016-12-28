package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetSchemas(t *testing.T) {
	Convey("Get schemas without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetSchemas, "TestGetSchemas")
	})

	Convey("Get schemas with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas?schema_name=public", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetSchemas, "TestGetSchemas")
	})

	Convey("Get schemas with custom ORDER BY clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas?schema_name=public&_order=schema_name", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetSchemas, "TestGetSchemas")
	})

	Convey("Get schemas with custom where clause and pagination", t, func() {
		r, err := http.NewRequest("GET", "/schemas?schema_name=public&_page=1&_page_size=20", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetSchemas, "TestGetSchemas")
	})
}
