package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/api"
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
		r, err := http.NewRequest("GET", "/schemas?schema_name=eq.public", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetSchemas, "TestGetSchemas")
	})

	Convey("Get schemas with custom ORDER BY clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas?schema_name=eq.public&_order=schema_name", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetSchemas, "TestGetSchemas")
	})

	Convey("Get schemas with custom where clause and pagination", t, func() {
		r, err := http.NewRequest("GET", "/schemas?schema_name=eq.public&_page=1&_page_size=20", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetSchemas, "TestGetSchemas")
	})

	Convey("Get schemas with COUNT clause", t, func() {
		r, err := http.NewRequest("GET", "/schemas?_count=*", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetSchemas, "TestGetSchemas")
	})

	Convey("Get schemas with custom where invalid clause", t, func() {
		router := mux.NewRouter()
		router.HandleFunc("/schemas", GetSchemas).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()

		r := api.Request{}
		doRequest(server.URL+"/schemas?0schema_name=eq.public", r, "GET", 400, "GetSchemas")
	})

	Convey("Get schemas with custom where and pagination invalid", t, func() {
		router := mux.NewRouter()
		router.HandleFunc("/schemas", GetSchemas).Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()

		r := api.Request{}
		doRequest(server.URL+"/schemas?schema_name=eq.public&_page=A", r, "GET", 400, "GetSchemas")
	})
}
