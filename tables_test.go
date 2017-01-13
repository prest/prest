package controllers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/config"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetTables(t *testing.T) {
	Convey("Get tables without custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/tables", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetTables, "TestGetTables1")
	})

	Convey("Get tables with custom where clause", t, func() {
		r, err := http.NewRequest("GET", "/tables?c.relname=test", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetTables, "TestGetTables2")
	})

	Convey("Get tables with custom where clause and pagination", t, func() {
		r, err := http.NewRequest("GET", "/tables?c.relname=test&_page=1&_page_size=20", nil)
		w := httptest.NewRecorder()
		So(err, ShouldBeNil)
		validate(w, r, GetTables, "TestGetTables3")
	})
}

func TestGetTablesByDatabaseAndSchema(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}", GetTablesByDatabaseAndSchema).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()
	Convey("Get tables by database and schema without custom where clause", t, func() {
		doValidGetRequest(server.URL+"/prest/public", "GetTablesByDatabaseAndSchema")
	})

	Convey("Get tables by database and schema with custom where clause", t, func() {
		doValidGetRequest(server.URL+"/prest/public?t.tablename=test", "GetTablesByDatabaseAndSchema")
	})

	Convey("Get tables by database and schema with custom where clause and pagination", t, func() {
		doValidGetRequest(server.URL+"/prest/public?t.tablename=test&_page=1&_page_size=20", "GetTablesByDatabaseAndSchema")
	})
}

func TestSelectFromTables(t *testing.T) {
	config.InitConf()
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", SelectFromTables).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()
	Convey("execute select in a table without custom where clause", t, func() {
		doValidGetRequest(server.URL+"/prest/public/test", "SelectFromTables")
	})
	Convey("execute select in a table with count all fields *", t, func() {
		doValidGetRequest(server.URL+"/prest/public/test?_count=*", "SelectFromTables")
	})
	Convey("execute select in a table with count function", t, func() {
		doValidGetRequest(server.URL+"/prest/public/test?_count=name", "SelectFromTables")
	})
	Convey("execute select in a table with custom where clause", t, func() {
		doValidGetRequest(server.URL+"/prest/public/test?name=nuveo", "SelectFromTables")
	})
	Convey("execute select in a table with custom join clause", t, func() {
		doValidGetRequest(server.URL+"/prest/public/test?_join=inner:test2:test2.name:eq:test.name", "SelectFromTables")
	})
	Convey("execute select in a table with custom where clause and pagination", t, func() {
		doValidGetRequest(server.URL+"/prest/public/test?name=nuveo&_page=1&_page_size=20", "SelectFromTables")
	})
	Convey("execute select in a table with select fields", t, func() {
		doValidGetRequest(server.URL+"/prest/public/test5?_select=celphone,name", "SelectFromTables")
	})
	Convey("execute select in a table with select *", t, func() {
		doValidGetRequest(server.URL+"/prest/public/test5?_select=*", "SelectFromTables")
	})
}

func TestInsertInTables(t *testing.T) {
	config.InitConf()
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", InsertInTables).Methods("POST")
	server := httptest.NewServer(router)
	defer server.Close()
	Convey("execute select in a table without custom where clause", t, func() {

		m := make(map[string]interface{}, 0)
		m["name"] = "prest"

		r := api.Request{
			Data: m,
		}

		doValidPostRequest(server.URL+"/prest/public/test", r, "InsertInTables")
	})
}

func TestDeleteFromTable(t *testing.T) {
	config.InitConf()
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", DeleteFromTable).Methods("DELETE")
	server := httptest.NewServer(router)
	defer server.Close()
	Convey("excute delete in a table without where clause", t, func() {
		doValidDeleteRequest(server.URL+"/prest/public/test", "DeleteFromTable")
	})
	Convey("excute delete in a table with where clause", t, func() {
		doValidDeleteRequest(server.URL+"/prest/public/test?name=nuveo", "DeleteFromTable")
	})
}

func TestUpdateFromTable(t *testing.T) {
	config.InitConf()
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", UpdateTable).Methods("PUT", "PATCH")
	server := httptest.NewServer(router)
	defer server.Close()

	m := make(map[string]interface{}, 0)
	m["name"] = "prest"

	r := api.Request{
		Data: m,
	}

	Convey("excute update in a table without where clause using PUT", t, func() {
		doValidPutRequest(server.URL+"/prest/public/test", r, "UpdateTable")
	})
	Convey("excute update in a table with where clause using PUT", t, func() {
		doValidPutRequest(server.URL+"/prest/public/test?name=nuveo", r, "UpdateTable")
	})
	Convey("excute update in a table without where clause using PATCH", t, func() {
		doValidPatchRequest(server.URL+"/prest/public/test", r, "UpdateTable")
	})
	Convey("excute update in a table with where clause using PATCH", t, func() {
		doValidPatchRequest(server.URL+"/prest/public/test?name=nuveo", r, "UpdateTable")
	})
}

func TestColumnsByRequest(t *testing.T) {
	Convey("Select fields from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=celphone", nil)
		So(err, ShouldBeNil)

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		So(selectStr, ShouldEqual, "celphone")
		So(len(selectQuery), ShouldEqual, 1)
	})
	Convey("Select all from table", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=*", nil)
		So(err, ShouldBeNil)

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		So(len(selectQuery), ShouldEqual, 1)
		So(selectStr, ShouldEqual, "*")
	})
	Convey("Try Select with empty '_select' field", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=", nil)
		So(err, ShouldBeNil)

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		So(len(selectQuery), ShouldEqual, 1)
		So(selectStr, ShouldEqual, "*")
	})
	Convey("Try Select with empty '_select' field", t, func() {
		r, err := http.NewRequest("GET", "/prest/public/test5?_select=celphone,battery", nil)
		So(err, ShouldBeNil)

		selectQuery := ColumnsByRequest(r)
		selectStr := strings.Join(selectQuery, ",")
		So(len(selectQuery), ShouldEqual, 2)
		So(selectStr, ShouldContainSubstring, "celphone,battery")
	})
}

func TestSelectFromViews(t *testing.T) {
	config.InitConf()
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/view/{view}", SelectFromViews).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	Convey("execute select in a view without custom where clause", t, func() {
		doValidGetRequest(server.URL+"/prest/public/view/view_test", "SelectFromViews")
	})

	Convey("execute select in a view with count all fields *", t, func() {
		doValidGetRequest(server.URL+"/prest/public/view/view_test?_count=*", "SelectFromViews")
	})

	Convey("execute select in a view with count function", t, func() {
		doValidGetRequest(server.URL+"/prest/public/view/view_test?_count=player", "SelectFromViews")
	})

	Convey("execute select in a view with custom where clause", t, func() {
		doValidGetRequest(server.URL+"/prest/public/view/view_test?player=gopher", "SelectFromViews")
	})

	Convey("execute select in a view with custom join clause", t, func() {
		doValidGetRequest(server.URL+"/prest/public/view/view_test?_join=inner:test2:test2.name:eq:view_test.player", "SelectFromViews")
	})

	Convey("execute select in a view with custom where clause and pagination", t, func() {
		doValidGetRequest(server.URL+"/prest/public/view/view_test?player=gopher&_page=1&_page_size=20", "SelectFromViews")
	})

	Convey("execute select in a view with select fields", t, func() {
		doValidGetRequest(server.URL+"/prest/public/view/view_test?_select=player", "SelectFromViews")
	})
}
