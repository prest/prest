package controllers

import (
	"net/http"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetDatabases(t *testing.T) {
	Convey("Get databases without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/databases", nil)
		So(err, ShouldBeNil)
		validate(r, GetDatabases)
	})

	Convey("Get databases with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/databases?datname=prest", nil)
		So(err, ShouldBeNil)
		validate(r, GetDatabases)
	})
}
