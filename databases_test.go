package controllers

import (
	"net/http"
	"testing"

	"net/http/httptest"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/api"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetDatabases(t *testing.T) {
	Convey("Get databases without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/databases", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetDatabases, "TestGetDatabases")
	})

	Convey("Get databases with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/databases?datname=$eq.prest", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetDatabases, "TestGetDatabases")
	})

	Convey("Get databases with custom order clause", t, func() {
		r, err := http.NewRequest("GET", "/databases?_order=datname", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetDatabases, "TestGetDatabases")
	})

	Convey("Get databases with custom where invalid clause", t, func() {
		router := mux.NewRouter()
		router.HandleFunc("/databases", GetDatabases).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()

		r := api.Request{}
		doRequest(server.URL+"/databases?0datname=prest", r, "GET", 400, "GetDatabases")
	})

	Convey("Get databases with custom where and pagination invalid", t, func() {
		router := mux.NewRouter()
		router.HandleFunc("/databases", GetDatabases).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()

		r := api.Request{}
		doRequest(server.URL+"/databases?datname=$eq.prest&_page=A", r, "GET", 400, "GetDatabases")
	})

	Convey("Get databases with custom where clause and pagination", t, func() {
		r, err := http.NewRequest("GET", "/databases?datname=$eq.prest&_page=1&_page_size=20", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetDatabases, "TestGetDatabases")
	})

	Convey("Get databases with COUNT clause", t, func() {
		r, err := http.NewRequest("GET", "/databases?_count=", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetDatabases, "TestGetDatabases")
	})
}
