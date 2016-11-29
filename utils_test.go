package controllers

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	. "github.com/smartystreets/goconvey/convey"
)

func validate(r *http.Request, h http.HandlerFunc) {
	w := httptest.NewRecorder()
	h(w, r)
	So(w.Code, ShouldEqual, 200)
	_, err := ioutil.ReadAll(w.Body)
	So(err, ShouldBeNil)
}
