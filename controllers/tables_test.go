package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetTables(t *testing.T) {
	Convey("Get tables without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/tables", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetTables)
	})

	Convey("Get tables with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/tables?c.relname=prest", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetTables)
	})
}

func TestGetTablesByDatabaseAndSchema(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}", GetTablesByDatabaseAndSchema).Methods("GET")

	Convey("Get tables by database and schema without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/prest/public", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		So(err, ShouldBeNil)
		validate(w, r, GetTablesByDatabaseAndSchema)
	})

	Convey("Get tables by database and schema with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/prest/public?t.tablename=test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		So(err, ShouldBeNil)
		validate(w, r, GetTablesByDatabaseAndSchema)
	})
}
