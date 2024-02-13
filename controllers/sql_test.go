// nolint
package controllers

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/adapters/mockgen"
	"github.com/prest/prest/config"
)

func TestExecuteScriptQuery(t *testing.T) {
	t.Parallel()

	var testCases = []struct {
		description string
		url         string
		method      string

		getScriptPath  string
		getScriptError error

		wantParseScript   bool
		parseScriptSQL    string
		parseScriptValues []interface{}
		parseScriptError  error

		wantExecScript  bool
		wantExecBytes   bool
		execScriptResp  string
		execScriptError error

		wantContains   string
		wantStatusCode int
	}{
		{
			description: "get script error",
			url:         "localhost:8080/testing/script-get/?field1=gopher",
			method:      "GET",

			getScriptPath:  "fulltable/get_all.sql",
			getScriptError: errors.New("get script error"),

			wantStatusCode: http.StatusBadRequest,
			wantContains:   "get script",
		},
		{
			description: "parse script error",
			url:         "localhost:8080/testing/script-get/?field1=gopher",
			method:      "GET",

			getScriptPath:  "",
			getScriptError: nil,

			wantParseScript:   true,
			parseScriptSQL:    "SELECT * FROM fulltable WHERE field1 = '{{.field1}}'",
			parseScriptValues: []interface{}{},
			parseScriptError:  errors.New("parse script error"),

			wantStatusCode: http.StatusBadRequest,
			wantContains:   "parse script",
		},
		{
			description: "execute script error",
			url:         "localhost:8080/testing/script-get/?field1=gopher",
			method:      "GET",

			getScriptPath:  "",
			getScriptError: nil,

			wantParseScript:   true,
			parseScriptSQL:    "SELECT * FROM fulltable WHERE field1 = '{{.field1}}'",
			parseScriptValues: []interface{}{},
			parseScriptError:  nil,

			wantExecScript:  true,
			wantExecBytes:   false,
			execScriptResp:  "execute script error",
			execScriptError: errors.New("execute script error"),

			wantStatusCode: http.StatusBadRequest,
			wantContains:   "execute script",
		},
		{
			description: "execute script success",
			url:         "localhost:8080/testing/script-get/?field1=gopher",
			method:      "POST", // avoid cache - dont want to setup test cache

			getScriptPath:  "",
			getScriptError: nil,

			wantParseScript:   true,
			parseScriptSQL:    "SELECT * FROM fulltable WHERE field1 = '{{.field1}}'",
			parseScriptValues: []interface{}{},
			parseScriptError:  nil,

			wantExecScript:  true,
			wantExecBytes:   true,
			execScriptResp:  "{}",
			execScriptError: nil,

			wantStatusCode: http.StatusOK,
			wantContains:   "{}",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Log(tc.description)

			ctrl := gomock.NewController(t)
			adapter := mockgen.NewMockAdapter(ctrl)

			ctrl2 := gomock.NewController(t)
			adapter2 := mockgen.NewMockScanner(ctrl2)

			// setup mocks
			adapter.EXPECT().GetCurrentConnDatabase().Return("postgres")
			adapter.EXPECT().SetCurrentConnDatabase("postgres")

			adapter.EXPECT().GetScript(tc.method, "", "").
				Return(tc.getScriptPath, tc.getScriptError)

			if tc.wantParseScript {
				adapter.EXPECT().ParseScript(tc.getScriptPath, gomock.Any()).
					Return(tc.parseScriptSQL, tc.parseScriptValues, tc.parseScriptError)
			}

			if tc.wantExecScript {
				if tc.wantExecBytes {
					adapter2.EXPECT().Bytes().Return([]byte(tc.execScriptResp))
				}

				adapter2.EXPECT().Err().Return(tc.execScriptError)

				adapter.EXPECT().ExecuteScriptsCtx(
					gomock.Any(), tc.method, tc.parseScriptSQL, gomock.Any()).
					Return(adapter2)
			}

			h := Config{
				server:  &config.Prest{PGDatabase: "postgres"},
				adapter: adapter}

			req := httptest.NewRequest(tc.method, tc.url, nil)
			recorder := httptest.NewRecorder()

			h.ExecuteFromScripts(recorder, req)

			resp := recorder.Result()
			require.Equal(t, tc.wantStatusCode, resp.StatusCode)
			require.Equal(t, "application/json; charset=utf-8", resp.Header.Get("Content-Type"))

			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(data), tc.wantContains)

		})

	}
}

// todo: move it to adapter
// func TestExecuteFromScripts(t *testing.T) {
// 	ctrl := gomock.NewController(t)
// 	adapter := mockgen.NewMockAdapter(ctrl)
// 	h := Config{adapter: adapter}

// 	router := mux.NewRouter()
// 	router.HandleFunc("/_QUERIES/{queriesLocation}/{script}", setHTTPTimeoutMiddleware(h.ExecuteFromScripts))
// 	server := httptest.NewServer(router)
// 	defer server.Close()

// 	var testCases = []struct {
// 		description string
// 		url         string
// 		method      string
// 		status      int
// 	}{
// 		{"Get results using scripts and funcs by GET method", "/_QUERIES/fulltable/funcs", "GET", http.StatusOK},
// 		{"Get results using scripts by GET method", "/_QUERIES/fulltable/get_all?field1=gopher", "GET", http.StatusOK},
// 		{"Get results using scripts by GET method (2)", "/_QUERIES/fulltable/get_header", "GET", http.StatusOK},
// 		{"Get results using scripts by POST method", "/_QUERIES/fulltable/write_all?field1=gopherzin&field2=pereira", "POST", http.StatusOK},
// 		{"Get results using scripts by PUT method", "/_QUERIES/fulltable/put_all?field1=trump&field2=pereira", "PUT", http.StatusOK},
// 		{"Get results using scripts by PATCH method", "/_QUERIES/fulltable/patch_all?field1=temer&field2=trump", "PATCH", http.StatusOK},
// 		{"Get results using scripts by DELETE method", "/_QUERIES/fulltable/delete_all?field1=trump", "DELETE", http.StatusOK},
// 		// errors
// 		{"Get errors using nonexistent folder", "/_QUERIES/fullnon/delete_all?field1=trump", "DELETE", http.StatusBadRequest},
// 		{"Get errors using nonexistent script", "/_QUERIES/fulltable/some_com_all?field1=trump", "DELETE", http.StatusBadRequest},
// 		{"Get errors with invalid execution of sql", "/_QUERIES/fulltable/create_table?field1=test7", "POST", http.StatusBadRequest},
// 	}

// 	for _, tc := range testCases {
// 		t.Log(tc.description)
// 		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "ExecuteFromScripts")
// 	}
// }

// todo: deprecate xml render
// func TestRenderWithXML(t *testing.T) {
// 	var testCases = []struct {
// 		description string
// 		url         string
// 		method      string
// 		status      int
// 		body        string
// 	}{
// 		{"Get schemas with COUNT clause with XML Render", "/schemas?_count=*&_renderer=xml", "GET", 200, "<objects><object><count>4</count></object></objects>"},
// 	}
// 	// todo: fix it
// 	ctrl := gomock.NewController(t)
// 	adapter := mockgen.NewMockAdapter(ctrl)
// 	h := Config{
// 		server:  &config.Prest{Debug: true},
// 		adapter: adapter,
// 	}

// 	n := middlewares.Get(&config.Prest{Debug: true}, nil) // todo: fix it
// 	r := mux.NewRouter()
// 	r.HandleFunc("/schemas", h.GetSchemas).Methods("GET")
// 	n.UseHandler(r)
// 	server := httptest.NewServer(n)
// 	defer server.Close()

// 	for _, tc := range testCases {
// 		t.Log(tc.description)
// 		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetSchemas", tc.body)

// 	}
// }
