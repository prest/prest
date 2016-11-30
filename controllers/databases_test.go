package controllers

import (
	"net/http"
	"testing"

	"net/http/httptest"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetDatabases(t *testing.T) {
	Convey("Get databases without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/databases", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetDatabases)
	})

	Convey("Get databases with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/databases?datname=prest", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetDatabases)
	})
}
