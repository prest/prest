package controllers

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	. "github.com/smartystreets/goconvey/convey"
)

func validate(r *http.Request) {
	w := httptest.NewRecorder()
	GetDatabases(w, r)
	So(w.Code, ShouldEqual, 200)
	_, err := ioutil.ReadAll(w.Body)
	So(err, ShouldBeNil)
}
