package controllers

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/config"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMain(m *testing.M) {
	config.InitConf()
	createMockScripts(config.PREST_CONF.QueriesPath)
	writeMockScripts(config.PREST_CONF.QueriesPath)

	code := m.Run()

	removeMockScripts(config.PREST_CONF.QueriesPath)
	os.Exit(code)
}

func TestExecuteFromScripts(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", ExecuteFromScripts)
	server := httptest.NewServer(router)
	defer server.Close()

	r := api.Request{}

	Convey("Get results using scripts by GET method", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/get_all?field1=gopher", r, "GET", 200, "ExecuteFromScripts")
	})

	Convey("Get results using scripts by POST method", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira", r, "POST", 200, "ExecuteFromScripts")
	})

	Convey("Get results using scripts by PUT method", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/put_all?field1=trump&field2=pereira", r, "PUT", 200, "ExecuteFromScripts")
	})

	Convey("Get results using scripts by PATCH method", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/patch_all?field1=temer&field2=trump", r, "PATCH", 200, "ExecuteFromScripts")
	})

	Convey("Get results using scripts by DELETE method", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/delete_all?field1=trump", r, "DELETE", 200, "ExecuteFromScripts")
	})

	Convey("Get results using scripts by DELETE method", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/delete_all?field1=trump", r, "DELETE", 200, "ExecuteFromScripts")
	})

	Convey("Get errors using nonexistent folder", t, func() {
		doRequest(server.URL+"/_QUERIES/fullnon/delete_all?field1=trump", r, "DELETE", 400, "ExecuteFromScripts")
	})

	Convey("Get errors using nonexistent script", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/remove_all?field1=trump", r, "DELETE", 400, "ExecuteFromScripts")
	})

	Convey("Get errors with invalid params in script", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/get_all?column1=gopher", r, "GET", 400, "ExecuteFromScripts")
	})

	Convey("Get errors with invalid execution of sql", t, func() {
		doRequest(server.URL+"/_QUERIES/fulltable/create_table?field1=test7", r, "POST", 400, "ExecuteFromScripts")
	})

}
