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
		r, err := http.NewRequest("GET", "/tables?c.relname=test", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetTables)
	})
}

func TestGetTablesByDatabaseAndSchema(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}", GetTablesByDatabaseAndSchema).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()
	Convey("Get tables by database and schema without custom where clause", t, func() {
		doValidRequest(server.URL + "/prest/public")
	})

	Convey("Get tables by database and schema with custom where clause", t, func() {
		doValidRequest(server.URL + "/prest/public?t.tablename=test")
	})
}

func TestSelectFromTable(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", SelectFromTables).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()
	Convey("execute select in a table without custom where clause", t, func() {
		doValidRequest(server.URL + "/prest/public")
	})

	Convey("execute select in a table with custom where clause", t, func() {
		doValidRequest(server.URL + "/prest/public/test?name=nuveo")
	})
}
