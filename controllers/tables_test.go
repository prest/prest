package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/adapters/mockgen"
	"github.com/prest/prest/config"
	pctx "github.com/prest/prest/context"
	"github.com/prest/prest/testutils"
)

func Test_GetTables(t *testing.T) {
	t.Parallel()

	var testCases = []struct {
		description string

		wherebyRequestSyntax string
		wherebyRequestValues []interface{}
		wherebyRequestErr    error

		wantTableWhere         bool
		wantTableWhereResp     string
		wantOrderByRequestResp string
		wantOrderByRequestErr  error

		tableOrderByResp string
		tableClauseResp  string
		wantDistinct     bool
		wantDistinctResp string
		wantDistinctErr  error

		wantQuery        bool
		wantQueryResp    bool
		wantQueryRespStr string
		wantQueryErr     error

		wantedResponseContains string
		wantStatus             int
	}{
		{
			description: "where clause error",

			wherebyRequestSyntax: "",
			wherebyRequestValues: nil,
			wherebyRequestErr:    dbErr,

			wantedResponseContains: dbErr.Error(),
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "order by request error",

			wherebyRequestSyntax: "syntax",
			wherebyRequestValues: []interface{}{},
			wherebyRequestErr:    nil,

			wantTableWhere:         true,
			wantTableWhereResp:     "table where response",
			wantOrderByRequestResp: "",
			wantOrderByRequestErr:  dbErr,

			wantedResponseContains: dbErr.Error(),
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "distinct error",

			wherebyRequestSyntax: "syntax",
			wherebyRequestValues: []interface{}{},
			wherebyRequestErr:    nil,

			wantTableWhere:         true,
			wantTableWhereResp:     "table where response",
			wantOrderByRequestResp: "",
			wantOrderByRequestErr:  nil,

			tableOrderByResp: "table order by response",
			tableClauseResp:  "table clause response",
			wantDistinct:     true,
			wantDistinctErr:  dbErr,

			wantedResponseContains: dbErr.Error(),
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "query error",

			wherebyRequestSyntax: "syntax",
			wherebyRequestValues: []interface{}{},
			wherebyRequestErr:    nil,

			wantTableWhere:         true,
			wantTableWhereResp:     "table where response",
			wantOrderByRequestResp: "",
			wantOrderByRequestErr:  nil,

			tableOrderByResp: "table order by response",
			tableClauseResp:  "table clause response",
			wantDistinct:     true,
			wantDistinctErr:  nil,

			wantQuery:        true,
			wantQueryResp:    false,
			wantQueryRespStr: "query response",
			wantQueryErr:     dbErr,

			wantedResponseContains: "check logs",
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "query ok",

			wherebyRequestSyntax: "syntax",
			wherebyRequestValues: []interface{}{},
			wherebyRequestErr:    nil,

			wantTableWhere:         true,
			wantTableWhereResp:     "table where response",
			wantOrderByRequestResp: "",
			wantOrderByRequestErr:  nil,

			tableOrderByResp: "table order by response",
			tableClauseResp:  "table clause response",
			wantDistinct:     true,
			wantDistinctErr:  nil,

			wantQuery:        true,
			wantQueryResp:    true,
			wantQueryRespStr: "query ok",
			wantQueryErr:     nil,

			wantedResponseContains: "query ok",
			wantStatus:             http.StatusOK,
		},
		// todo: verify adapter has these cases
		// {"Get tables without custom where clause", "/tables", "GET", http.StatusOK},
		// {"Get tables with custom where clause", "/tables?c.relname=$eq.test", "GET", http.StatusOK},
		// {"Get tables with custom order clause", "/tables?_order=c.relname", "GET", http.StatusOK},
		// {"Get tables with custom where clause and pagination", "/tables?c.relname=$eq.test&_page=1&_page_size=20", "GET", http.StatusOK},
		// {"Get tables with COUNT clause", "/tables?_count=*", "GET", http.StatusOK},
		// {"Get tables with distinct clause", "/tables?_distinct=true", "GET", http.StatusOK},
		// {"Get tables with custom where invalid clause", "/tables?0c.relname=$eq.test", "GET", http.StatusBadRequest},
		// {"Get tables with ORDER BY and invalid column", "/tables?_order=0c.relname", "GET", http.StatusBadRequest},
		// {"Get tables with noexistent column", "/tables?c.rolooo=$eq.test", "GET", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			t.Log(tc.description)

			ctrl := gomock.NewController(t)
			adapter := mockgen.NewMockAdapter(ctrl)

			ctrl2 := gomock.NewController(t)
			adapter2 := mockgen.NewMockScanner(ctrl2)

			adapter.EXPECT().WhereByRequest(gomock.Any(), 1).
				Return(tc.wherebyRequestSyntax, tc.wherebyRequestValues, tc.wherebyRequestErr)

			if tc.wantTableWhere {
				adapter.EXPECT().TableWhere(tc.wherebyRequestSyntax).
					Return(tc.wantTableWhereResp)

				adapter.EXPECT().OrderByRequest(gomock.Any()).Return(
					tc.wantOrderByRequestResp, tc.wantOrderByRequestErr)
			}

			if tc.wantDistinct {
				adapter.EXPECT().TableOrderBy(gomock.Any()).Return(tc.tableOrderByResp)

				adapter.EXPECT().TableClause().Return(tc.tableClauseResp)

				adapter.EXPECT().DistinctClause(gomock.Any()).Return(
					tc.wantDistinctResp, tc.wantDistinctErr)
			}

			if tc.wantQuery {
				adapter.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(adapter2)

				adapter2.EXPECT().Err().Return(tc.wantQueryErr)
			}

			if tc.wantQueryResp {
				adapter2.EXPECT().Bytes().Return([]byte(tc.wantQueryRespStr))
			}

			h := Config{
				server:  &config.Prest{Debug: true},
				adapter: adapter,
			}

			req := httptest.NewRequest(http.MethodGet, "localhost:8080", nil)

			recorder := httptest.NewRecorder()

			h.GetTables(recorder, req)

			resp := recorder.Result()
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.Equal(t,
				"application/json; charset=utf-8", resp.Header.Get("Content-Type"))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), tc.wantedResponseContains)
		})
	}
}

func Test_GetTablesByDatabaseAndSchema(t *testing.T) {
	t.Parallel()

	var testCases = []struct {
		description string
		url         string

		wantSingleDB        bool
		wantDifferentDBResp string

		wantWherByRequest    bool
		wherebyRequestSyntax string
		wherebyRequestValues []interface{}
		wherebyRequestErr    error

		wantOrderBy                bool
		wantSchemaTablesWhereResp  string
		wantSchemaTablesClauseResp string
		wantOrderByRequestResp     string
		wantOrderByRequestErr      error

		wantPaginate                bool
		wantSchemaTablesOrderByResp string
		wantPaginateResp            string
		wantPaginateErr             error

		wantQuery        bool
		wantQueryResp    bool
		wantQueryRespStr string
		wantQueryErr     error

		wantedResponseContains string
		wantStatus             int
	}{
		{
			description: "different db error",
			url:         "localhost:8080/prest-test/public",

			wantSingleDB:        true,
			wantDifferentDBResp: "prest-test",

			wantedResponseContains: ErrDatabaseNotAllowed.Error(),
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "where by request error",
			url:         "localhost:8080/prest-test/public",

			wantSingleDB:        false,
			wantDifferentDBResp: "prest-test",

			wantWherByRequest:    true,
			wherebyRequestSyntax: "",
			wherebyRequestValues: nil,
			wherebyRequestErr:    dbErr,

			wantedResponseContains: dbErr.Error(),
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "order by request error",
			url:         "localhost:8080/prest-test/public",

			wantSingleDB:        false,
			wantDifferentDBResp: "prest-test",

			wantWherByRequest:    true,
			wherebyRequestSyntax: "syntax",
			wherebyRequestValues: []interface{}{},
			wherebyRequestErr:    nil,

			wantOrderBy:                true,
			wantSchemaTablesWhereResp:  "schema tables where response",
			wantSchemaTablesClauseResp: "schema tables clause response",
			wantOrderByRequestResp:     "",
			wantOrderByRequestErr:      dbErr,

			wantedResponseContains: dbErr.Error(),
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "paginate error",
			url:         "localhost:8080/prest-test/public",

			wantSingleDB:        false,
			wantDifferentDBResp: "prest-test",

			wantWherByRequest:    true,
			wherebyRequestSyntax: "syntax",
			wherebyRequestValues: []interface{}{},
			wherebyRequestErr:    nil,

			wantOrderBy:                true,
			wantSchemaTablesWhereResp:  "schema tables where response",
			wantSchemaTablesClauseResp: "schema tables clause response",
			wantOrderByRequestResp:     "",
			wantOrderByRequestErr:      nil,

			wantPaginate:                true,
			wantSchemaTablesOrderByResp: "schema tables order by response",
			wantPaginateResp:            "paginate response",
			wantPaginateErr:             errors.New("paginate error"),

			wantedResponseContains: "paginate error",
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "query error",
			url:         "localhost:8080/prest-test/public",

			wantSingleDB:        false,
			wantDifferentDBResp: "prest-test",

			wantWherByRequest:    true,
			wherebyRequestSyntax: "syntax",
			wherebyRequestValues: []interface{}{},
			wherebyRequestErr:    nil,

			wantOrderBy:                true,
			wantSchemaTablesWhereResp:  "schema tables where response",
			wantSchemaTablesClauseResp: "schema tables clause response",
			wantOrderByRequestResp:     "",
			wantOrderByRequestErr:      nil,

			wantPaginate:                true,
			wantSchemaTablesOrderByResp: "schema tables order by response",
			wantPaginateResp:            "paginate response",
			wantPaginateErr:             nil,

			wantQuery:        true,
			wantQueryResp:    false,
			wantQueryRespStr: "query response",
			wantQueryErr:     errors.New("query error"),

			wantedResponseContains: ErrCouldNotPerformQuery.Error(),
			wantStatus:             http.StatusBadRequest,
		},
		{
			description: "query ok",
			url:         "localhost:8080/prest-test/public",

			wantSingleDB:        false,
			wantDifferentDBResp: "prest-test",

			wantWherByRequest:    true,
			wherebyRequestSyntax: "syntax",
			wherebyRequestValues: []interface{}{},
			wherebyRequestErr:    nil,

			wantOrderBy:                true,
			wantSchemaTablesWhereResp:  "schema tables where response",
			wantSchemaTablesClauseResp: "schema tables clause response",
			wantOrderByRequestResp:     "",
			wantOrderByRequestErr:      nil,

			wantPaginate:                true,
			wantSchemaTablesOrderByResp: "schema tables order by response",
			wantPaginateResp:            "paginate response",
			wantPaginateErr:             nil,

			wantQuery:        true,
			wantQueryResp:    true,
			wantQueryRespStr: "query response",
			wantQueryErr:     nil,

			wantedResponseContains: "query response",
			wantStatus:             http.StatusOK,
		},
		// todo: verify adapter has these cases
		// {"Get tables by database and schema without custom where clause", "/prest-test/public", "GET", http.StatusOK},
		// {"Get tables by database and schema with custom where clause", "/prest-test/public?t.tablename=$eq.test", "GET", http.StatusOK},
		// {"Get tables by database and schema with order clause", "/prest-test/public?t.tablename=$eq.test&_order=t.tablename", "GET", http.StatusOK},
		// {"Get tables by database and schema with custom where clause and pagination", "/prest-test/public?t.tablename=$eq.test&_page=1&_page_size=20", "GET", http.StatusOK},
		// {"Get tables by database and schema with distinct clause", "/prest-test/public?_distinct=true", "GET", http.StatusOK},
		// errors
		// {"Get tables by database and schema with custom where invalid clause", "/prest-test/public?0t.tablename=$eq.test", "GET", http.StatusBadRequest},
		// {"Get tables by databases and schema with custom where and pagination invalid", "/prest-test/public?t.tablename=$eq.test&_page=A&_page_size=20", "GET", http.StatusBadRequest},
		// {"Get tables by databases and schema with ORDER BY and column invalid", "/prest-test/public?_order=0t.tablename", "GET", http.StatusBadRequest},
		// {"Get tables by databases with noexistent column", "/prest-test/public?t.taababa=$eq.test", "GET", http.StatusBadRequest},
		// {"Get tables by databases with not configured database", "/random/public?t.taababa=$eq.test", "GET", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			t.Log(tc.description)

			ctrl := gomock.NewController(t)
			adapter := mockgen.NewMockAdapter(ctrl)

			ctrl2 := gomock.NewController(t)
			adapter2 := mockgen.NewMockScanner(ctrl2)

			adapter.EXPECT().GetCurrentConnDatabase().Return(tc.wantDifferentDBResp)

			if tc.wantWherByRequest {
				adapter.EXPECT().WhereByRequest(gomock.Any(), 3).
					Return(tc.wherebyRequestSyntax, tc.wherebyRequestValues, tc.wherebyRequestErr)
			}

			if tc.wantOrderBy {
				adapter.EXPECT().SchemaTablesWhere(gomock.Any()).
					Return(tc.wantSchemaTablesWhereResp)

				adapter.EXPECT().SchemaTablesClause().Return(
					tc.wantSchemaTablesClauseResp)

				adapter.EXPECT().OrderByRequest(gomock.Any()).Return(
					tc.wantOrderByRequestResp, tc.wantOrderByRequestErr)
			}

			if tc.wantPaginate {
				adapter.EXPECT().SchemaTablesOrderBy(gomock.Any()).
					Return(tc.wantSchemaTablesOrderByResp)

				adapter.EXPECT().PaginateIfPossible(gomock.Any()).
					Return(tc.wantPaginateResp, tc.wantPaginateErr)
			}

			if tc.wantQuery {
				adapter.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(adapter2)

				adapter2.EXPECT().Err().Return(tc.wantQueryErr)
			}

			if tc.wantQueryResp {
				adapter2.EXPECT().Bytes().Return([]byte(tc.wantQueryRespStr))
			}

			h := Config{
				server:  &config.Prest{SingleDB: tc.wantSingleDB},
				adapter: adapter,
			}

			request := httptest.NewRequest(http.MethodGet, tc.url, nil)

			recorder := httptest.NewRecorder()

			h.GetTablesByDatabaseAndSchema(recorder, request)

			resp := recorder.Result()
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.Equal(t,
				"application/json; charset=utf-8", resp.Header.Get("Content-Type"))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), tc.wantedResponseContains)
		})
	}
}

func TestSelectFromTables(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{
		server:  &config.Prest{Debug: true},
		adapter: adapter,
	}

	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(h.SelectFromTables)).
		Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        string
	}{
		{"execute select in a table with array", "/prest-test/public/testarray", "GET", http.StatusOK, "[{\"id\": 100, \"data\": [\"Gohan\", \"Goten\"]}]"},
		{"execute select in a table without custom where clause", "/prest-test/public/test", "GET", http.StatusOK, ""},
		{"execute select in a table case sentive", "/prest-test/public/Reply", "GET", http.StatusOK, "[{\"id\": 1, \"name\": \"prest tester\"}, {\"id\": 2, \"name\": \"prest-test-insert\"}, {\"id\": 3, \"name\": \"prest-test-insert-ctx\"}, {\"id\": 4, \"name\": \"3prest-test-batch-insert\"}, {\"id\": 5, \"name\": \"3batch-prest-test-insert\"}, {\"id\": 6, \"name\": \"3prest-test-batch-insert-ctx\"}, {\"id\": 7, \"name\": \"3batch-prest-test-insert-ctx\"}, {\"id\": 8, \"name\": \"copy-ctx\"}, {\"id\": 9, \"name\": \"copy-ctx\"}, {\"id\": 10, \"name\": \"copy\"}, {\"id\": 11, \"name\": \"copy\"}]"},
		{"execute select in a table with count all fields *", "/prest-test/public/test?_count=*", "GET", http.StatusOK, ""},
		{"execute select in a table with count function", "/prest-test/public/test?_count=name", "GET", http.StatusOK, ""},
		{"execute select in a table with custom where clause", "/prest-test/public/test?name=$eq.test", "GET", http.StatusOK, ""},
		{"execute select in a table with custom join clause", "/prest-test/public/test?_join=inner:test8:test8.nameforjoin:$eq:test.name", "GET", http.StatusOK, ""},
		{"execute select in a table with order clause empty", "/prest-test/public/test?_order=", "GET", http.StatusOK, ""},
		{"execute select in a table with custom where clause and pagination", "/prest-test/public/test?name=$eq.test&_page=1&_page_size=20", "GET", http.StatusOK, ""},
		{"execute select in a table with select fields", "/prest-test/public/test5?_select=celphone,name", "GET", http.StatusOK, ""},
		{"execute select in a table with select *", "/prest-test/public/test5?_select=*", "GET", http.StatusOK, ""},
		{"execute select in a table with select * and distinct", "/prest-test/public/test5?_select=*&_distinct=true", "GET", http.StatusOK, ""},

		{"execute select in a table with group by clause", "/prest-test/public/test_group_by_table?_select=age,sum:salary&_groupby=age", "GET", http.StatusOK, ""},
		{"execute select in a table with group by and having clause", "/prest-test/public/test_group_by_table?_select=age,sum:salary&_groupby=age->>having:sum:salary:$gt:3000", "GET", http.StatusOK, "[{\"age\": 19, \"sum\": 7997}]"},

		{"execute select in a view without custom where clause", "/prest-test/public/view_test", "GET", http.StatusOK, ""},
		{"execute select in a view with count all fields *", "/prest-test/public/view_test?_count=*", "GET", http.StatusOK, ""},
		{"execute select in a view with count function", "/prest-test/public/view_test?_count=player", "GET", http.StatusOK, ""},
		{"execute select in a view with count function check return list", "/prest-test/public/view_test?_count=player", "GET", http.StatusOK, "[{\"count\": 1}]"},
		{"execute select in a view with count function check return object (_count_first)", "/prest-test/public/view_test?_count=player&_count_first=true", "GET", http.StatusOK, "{\"count\":1}"},
		{"execute select in a view with order function", "/prest-test/public/view_test?_order=-player", "GET", http.StatusOK, ""},
		{"execute select in a view with custom where clause", "/prest-test/public/view_test?player=$eq.gopher", "GET", http.StatusOK, ""},
		{"execute select in a view with custom join clause", "/prest-test/public/view_test?_join=inner:test2:test2.name:eq:view_test.player", "GET", http.StatusOK, ""},
		{"execute select in a view with custom where clause and pagination", "/prest-test/public/view_test?player=$eq.gopher&_page=1&_page_size=20", "GET", http.StatusOK, ""},
		{"execute select in a view with select fields", "/prest-test/public/view_test?_select=player", "GET", http.StatusOK, ""},

		{"execute select in a table with invalid join clause", "/prest-test/public/test?_join=inner:test2:test2.name", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid where clause", "/prest-test/public/test?0name=$eq.test", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with order clause and column invalid", "/prest-test/public/test?_order=0name", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid pagination clause", "/prest-test/public/test?name=$eq.test&_page=A", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid where clause", "/prest-test/public/test?0name=$eq.test", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid count clause", "/prest-test/public/test?_count=0name", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid order clause", "/prest-test/public/test?_order=0name", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid fields using group by clause", "/prest-test/public/test_group_by_table?_select=pa,sum:pum&_groupby=pa", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid fields using group by and having clause", "/prest-test/public/test_group_by_table?_select=pa,sum:pum&_groupby=pa->>having:sum:pmu:$eq:150", "GET", http.StatusBadRequest, ""},

		{"execute select in a view with an other column", "/prest-test/public/view_test?_select=celphone", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with where and column invalid", "/prest-test/public/view_test?0celphone=$eq.888888", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with custom join clause invalid", "/prest-test/public/view_test?_join=inner:test2.name:eq:view_test.player", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with custom where clause and pagination invalid", "/prest-test/public/view_test?player=$eq.gopher&_page=A&_page_size=20", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with order by and column invalid", "/prest-test/public/view_test?_order=0celphone", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with count column invalid", "/prest-test/public/view_test?_count=0celphone", "GET", http.StatusBadRequest, ""},

		{"execute select in a db that does not exist", "/invalid/public/view_test?_count=0celphone", "GET", http.StatusBadRequest, ""},
	}

	for _, tc := range testCases {
		t.Log(tc.description)

		if tc.body != "" {
			testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "SelectFromTables", tc.body)
			continue
		}
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "SelectFromTables")
	}
}

func TestInsertInTables(t *testing.T) {
	m := make(map[string]interface{})
	m["name"] = "prest-test"

	mJSON := make(map[string]interface{})
	mJSON["name"] = "prest-test"
	mJSON["data"] = `{"term": "name", "subterm": ["names", "of", "subterms"], "obj": {"emp": "prestd"}}`

	mARRAY := make(map[string]interface{})
	mARRAY["data"] = []string{"value 1", "value 2", "value 3"}

	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{
		server:  &config.Prest{Debug: true},
		adapter: adapter,
	}

	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(h.InsertInTables)).
		Methods("POST")
	server := httptest.NewServer(router)
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		request     map[string]interface{}
		status      int
	}{
		{"execute insert in a table with array field", "/prest-test/public/testarray", mARRAY, http.StatusCreated},
		{"execute insert in a table with jsonb field", "/prest-test/public/testjson", mJSON, http.StatusCreated},
		{"execute insert in a table without custom where clause", "/prest-test/public/test", m, http.StatusCreated},
		{"execute insert in a table with invalid database", "/0prest-test/public/test", m, http.StatusBadRequest},
		{"execute insert in a table with invalid schema", "/prest-test/0public/test", m, http.StatusNotFound},
		{"execute insert in a table with invalid table", "/prest-test/public/0test", m, http.StatusNotFound},
		{"execute insert in a table with invalid body", "/prest-test/public/test", nil, http.StatusBadRequest},

		{"execute insert in a database that does not exist", "/invalid/public/0test", m, http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, tc.request, "POST", tc.status, "InsertInTables")
	}
}

func TestBatchInsertInTables(t *testing.T) {
	m := make([]map[string]interface{}, 0)
	m = append(m, map[string]interface{}{"name": "bprest"}, map[string]interface{}{"name": "aprest"})

	mJSON := make([]map[string]interface{}, 0)
	mJSON = append(mJSON, map[string]interface{}{"name": "cprest", "data": `{"term": "name", "subterm": ["names", "of", "subterms"], "obj": {"emp": "prestd"}}`}, map[string]interface{}{"name": "dprest", "data": `{"term": "name", "subterms": ["names", "of", "subterms"], "obj": {"emp": "prestd"}}`})

	mARRAY := make([]map[string]interface{}, 0)
	mARRAY = append(mARRAY, map[string]interface{}{"data": []string{"1", "2"}}, map[string]interface{}{"data": []string{"1", "2", "3"}})

	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{
		server:  &config.Prest{Debug: true},
		adapter: adapter,
	}

	router := mux.NewRouter()
	router.HandleFunc("/batch/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(h.BatchInsertInTables)).
		Methods("POST")
	server := httptest.NewServer(router)
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		request     []map[string]interface{}
		status      int
		isCopy      bool
	}{
		{"execute insert in a table with array field", "/batch/prest-test/public/testarray", mARRAY, http.StatusCreated, false},
		{"execute insert in a table with jsonb field", "/batch/prest-test/public/testjson", mJSON, http.StatusCreated, false},
		{"execute insert in a table without custom where clause", "/batch/prest-test/public/test", m, http.StatusCreated, false},
		{"execute insert in a table with invalid database", "/batch/0prest-test/public/test", m, http.StatusBadRequest, false},
		{"execute insert in a table with invalid schema", "/batch/prest-test/0public/test", m, http.StatusNotFound, false},
		{"execute insert in a table with invalid table", "/batch/prest-test/public/0test", m, http.StatusNotFound, false},
		{"execute insert in a table with invalid body", "/batch/prest-test/public/test", nil, http.StatusBadRequest, false},
		{"execute insert in a table with array field with copy", "/batch/prest-test/public/testarray", mARRAY, http.StatusCreated, true},
		{"execute insert in a table with jsonb field with copy", "/batch/prest-test/public/testjson", mJSON, http.StatusCreated, true},

		{"execute insert in a db that does not exist", "/batch/invalid/public/test", nil, http.StatusBadRequest, false},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			byt, err := json.Marshal(tc.request)
			if err != nil {
				t.Error("error on json marshal", err)
			}
			req, err := http.NewRequest(http.MethodPost, server.URL+tc.url, bytes.NewReader(byt))
			if err != nil {
				t.Error("error on New Request", err)
			}
			if tc.isCopy {
				req.Header.Set("Prest-Batch-Method", "copy")
			}
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Error("error on Do Request", err)
			}
			if resp.StatusCode != tc.status {
				t.Errorf("expected %d, got: %d", tc.status, resp.StatusCode)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Error("error on io ReadAll", err)
			}
			if tc.isCopy && len(body) != 0 {
				t.Errorf("len body is %d", len(body))
			}
		})
	}
}

func TestDeleteFromTable(t *testing.T) {

	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{
		server:  &config.Prest{Debug: true},
		adapter: adapter,
	}

	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(h.DeleteFromTable)).
		Methods("DELETE")
	server := httptest.NewServer(router)
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		request     map[string]interface{}
		status      int
	}{
		{"execute delete in a table without custom where clause", "/prest-test/public/test", nil, http.StatusOK},
		{"excute delete in a table with where clause", "/prest-test/public/test?name=$eq.test", nil, http.StatusOK},
		{"execute delete in a table with invalid database", "/0prest-test/public/test", nil, http.StatusBadRequest},
		{"execute delete in a table with invalid schema", "/prest-test/0public/test", nil, http.StatusNotFound},
		{"execute delete in a table with invalid table", "/prest-test/public/0test", nil, http.StatusNotFound},
		{"execute delete in a table with invalid where clause", "/prest-test/public/test?0name=$eq.nuveo", nil, http.StatusBadRequest},

		{"execute delete in a invalid db", "/invalid/public/0test", nil, http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, tc.request, "DELETE", tc.status, "DeleteFromTable")
	}
}

func TestUpdateFromTable(t *testing.T) {

	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{
		server:  &config.Prest{Debug: true},
		adapter: adapter,
	}

	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(h.UpdateTable)).
		Methods("PUT", "PATCH")
	server := httptest.NewServer(router)
	defer server.Close()

	m := make(map[string]interface{})
	m["name"] = "prest"

	var testCases = []struct {
		description string
		url         string
		request     map[string]interface{}
		status      int
	}{
		{"execute update in a table without custom where clause", "/prest-test/public/test", m, http.StatusOK},
		{"execute update in a table with where clause", "/prest-test/public/test?name=$eq.test", m, http.StatusOK},
		{"execute update in a table with where clause and returning all fields", "/prest-test/public/test?id=1&_returning=*", m, http.StatusOK},
		{"execute update in a table with where clause and returning name field", "/prest-test/public/test?id=2&_returning=name", m, http.StatusOK},
		{"execute update in a table with invalid database", "/0prest-test/public/test", m, http.StatusBadRequest},
		{"execute update in a table with invalid schema", "/prest-test/0public/test", m, http.StatusNotFound},
		{"execute update in a table with invalid table", "/prest-test/public/0test", m, http.StatusNotFound},
		{"execute update in a table with invalid where clause", "/prest-test/public/test?0name=$eq.nuveo", m, http.StatusBadRequest},
		{"execute update in a table with invalid body", "/prest-test/public/test?name=$eq.nuveo", nil, http.StatusBadRequest},

		{"execute update in a invalid db", "/invalid/public/test", m, http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)

		testutils.DoRequest(t, server.URL+tc.url, tc.request, "PUT", tc.status, "UpdateTable")
		testutils.DoRequest(t, server.URL+tc.url, tc.request, "PATCH", tc.status, "UpdateTable")
	}
}

func TestShowTable(t *testing.T) {

	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)
	h := Config{
		server:  &config.Prest{Debug: true},
		adapter: adapter,
	}

	router := mux.NewRouter()
	router.HandleFunc("/show/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(h.ShowTable)).
		Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"execute select in a table test custom information table", "/show/prest-test/public/test", "GET", http.StatusOK},
		{"execute select in a table test2 custom information table", "/show/prest-test/public/test2", "GET", http.StatusOK},
		{"execute select in a invalid db", "/show/invalid/public/test2", "GET", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "ShowTable")
	}
}

func setHTTPTimeoutMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pctx.HTTPTimeoutKey, 60))) // nolint
	}
}

func Test_DifferentDbQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := mockgen.NewMockAdapter(ctrl)
	adapter.EXPECT().GetCurrentConnDatabase().Return("db1").Times(3)
	h := Config{
		server: &config.Prest{
			Debug:    true,
			SingleDB: true,
		},
		adapter: adapter,
	}

	cases := []struct {
		description string
		database    string
		singleDB    bool
		expected    bool
	}{
		{
			description: "Same database",
			database:    "db1",
			singleDB:    true,
			expected:    false,
		},
		{
			description: "Different database",
			database:    "db2",
			singleDB:    true,
			expected:    true,
		},
		{
			description: "Different database multiple config",
			database:    "db2",
			singleDB:    false,
			expected:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			t.Log(tc.description)
			if !tc.singleDB {
				h.server.SingleDB = tc.singleDB
			}
			result := h.differentDbQuery(tc.database)
			require.Equal(t, tc.expected, result)
		})
	}
}
