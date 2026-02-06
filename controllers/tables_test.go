package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/testutils"

	"github.com/gorilla/mux"
)

// Should be in sync with databases under test (see `testdata/runtest.sh` and
// Github `test` workflow)
var databases = []string{"prest-test", "secondary-db"}

func Init() {
	config.Load()
	postgres.Load()
	if config.PrestConf.PGDatabase != "prest-test" {
		slog.Error("expected db: 'prest-test'", "got", config.PrestConf.PGDatabase)
		os.Exit(1)
	}
	if config.PrestConf.Adapter.GetDatabase() != "prest-test" {
		slog.Error("expected Adapter db: 'prest-test'", "got", config.PrestConf.Adapter.GetDatabase())
		os.Exit(1)
	}
}

func TestGetTables(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Get tables without custom where clause", "/tables", "GET", http.StatusOK},
		{"Get tables with custom where clause", "/tables?c.relname=$eq.test", "GET", http.StatusOK},
		{"Get tables with custom order clause", "/tables?_order=c.relname", "GET", http.StatusOK},
		{"Get tables with custom where clause and pagination", "/tables?c.relname=$eq.test&_page=1&_page_size=20", "GET", http.StatusOK},
		{"Get tables with COUNT clause", "/tables?_count=*", "GET", http.StatusOK},
		{"Get tables with distinct clause", "/tables?_distinct=true", "GET", http.StatusOK},
		{"Get tables with custom where invalid clause", "/tables?0c.relname=$eq.test", "GET", http.StatusBadRequest},
		{"Get tables with ORDER BY and invalid column", "/tables?_order=0c.relname", "GET", http.StatusBadRequest},
		{"Get tables with noexistent column", "/tables?c.rolooo=$eq.test", "GET", http.StatusBadRequest},
	}

	router := mux.NewRouter()
	router.HandleFunc("/tables", setHTTPTimeoutMiddleware(GetTables)).
		Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tc := range testCases {
		t.Log(tc.description)
		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "GetTables")
	}
}

func TestGetTablesByDatabaseAndSchema(t *testing.T) {
	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
	}{
		{"Get tables by database and schema without custom where clause", "/%s/public", "GET", http.StatusOK},
		{"Get tables by database and schema with custom where clause", "/%s/public?t.tablename=$eq.test", "GET", http.StatusOK},
		{"Get tables by database and schema with order clause", "/%s/public?t.tablename=$eq.test&_order=t.tablename", "GET", http.StatusOK},
		{"Get tables by database and schema with custom where clause and pagination", "/%s/public?t.tablename=$eq.test&_page=1&_page_size=20", "GET", http.StatusOK},
		{"Get tables by database and schema with distinct clause", "/%s/public?_distinct=true", "GET", http.StatusOK},
		// errors
		{"Get tables by database and schema with custom where invalid clause", "/%s/public?0t.tablename=$eq.test", "GET", http.StatusBadRequest},
		{"Get tables by databases and schema with custom where and pagination invalid", "/%s/public?t.tablename=$eq.test&_page=A&_page_size=20", "GET", http.StatusBadRequest},
		{"Get tables by databases and schema with ORDER BY and column invalid", "/%s/public?_order=0t.tablename", "GET", http.StatusBadRequest},
		{"Get tables by databases with noexistent column", "/%s/public?t.taababa=$eq.test", "GET", http.StatusBadRequest},
		{"Get tables by databases with not configured database", "/random/public?t.taababa=$eq.test", "GET", http.StatusBadRequest},
	}

	// Re-initialize pREST instance under test, mostly to revert `config` changes below
	defer Init()

	for _, db := range databases {
		// Testing against multiple databases needs `SingleDB = false` in the
		// config
		config.PrestConf.SingleDB = false
		router := mux.NewRouter()
		router.HandleFunc("/{database}/{schema}", setHTTPTimeoutMiddleware(GetTablesByDatabaseAndSchema)).
			Methods("GET")
		server := httptest.NewServer(router)
		defer server.Close()
		for _, tc := range testCases {
			t.Log(fmt.Sprintf("(DB: %s) %s", db, tc.description))
			testutils.DoRequest(t, fmt.Sprintf(server.URL+tc.url, db), nil, tc.method, tc.status, "GetTablesByDatabaseAndSchema")
		}
	}
}

func TestSelectFromTables(t *testing.T) {
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(SelectFromTables)).
		Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()
	defer Init()

	var testCases = []struct {
		description string
		url         string
		method      string
		status      int
		body        string
	}{
		{"execute select in a table with array", "/%s/public/testarray", "GET", http.StatusOK, "[{\"id\": 100, \"data\": [\"Gohan\", \"Goten\"]}]"},
		{"execute select in a table without custom where clause", "/%s/public/test", "GET", http.StatusOK, ""},
		{"execute select in a table case sentive", "/%s/public/Reply", "GET", http.StatusOK, "[{\"id\": 1, \"name\": \"prest tester\"}]"},
		{"execute select in a table with count all fields *", "/%s/public/test?_count=*", "GET", http.StatusOK, ""},
		{"execute select in a table with count function", "/%s/public/test?_count=name", "GET", http.StatusOK, ""},
		{"execute select in a table with custom where clause", "/%s/public/test?name=$eq.test", "GET", http.StatusOK, ""},
		{"execute select in a table with custom join clause", "/%s/public/test?_join=inner:test8:test8.nameforjoin:$eq:test.name", "GET", http.StatusOK, ""},
		{"execute select in a table with order clause empty", "/%s/public/test?_order=", "GET", http.StatusOK, ""},
		{"execute select in a table with custom where clause and pagination", "/%s/public/test?name=$eq.test&_page=1&_page_size=20", "GET", http.StatusOK, ""},
		{"execute select in a table with select fields", "/%s/public/test5?_select=celphone,name", "GET", http.StatusOK, ""},
		{"execute select in a table with select *", "/%s/public/test5?_select=*", "GET", http.StatusOK, ""},
		{"execute select in a table with select * and distinct", "/%s/public/test5?_select=*&_distinct=true", "GET", http.StatusOK, ""},

		{"execute select in a table with group by clause", "/%s/public/test_group_by_table?_select=age,sum:salary&_groupby=age", "GET", http.StatusOK, ""},
		{"execute select in a table with group by and having clause", "/%s/public/test_group_by_table?_select=age,sum:salary&_groupby=age->>having:sum:salary:$gt:3000", "GET", http.StatusOK, "[{\"age\": 19, \"sum\": 7997}]"},

		{"execute select in a view without custom where clause", "/%s/public/view_test", "GET", http.StatusOK, ""},
		{"execute select in a view with count all fields *", "/%s/public/view_test?_count=*", "GET", http.StatusOK, ""},
		{"execute select in a view with count function", "/%s/public/view_test?_count=player", "GET", http.StatusOK, ""},
		{"execute select in a view with count function check return list", "/%s/public/view_test?_count=player", "GET", http.StatusOK, "[{\"count\": 1}]"},
		{"execute select in a view with count function check return object (_count_first)", "/%s/public/view_test?_count=player&_count_first=true", "GET", http.StatusOK, "{\"count\":1}"},
		{"execute select in a view with order function", "/%s/public/view_test?_order=-player", "GET", http.StatusOK, ""},
		{"execute select in a view with custom where clause", "/%s/public/view_test?player=$eq.gopher", "GET", http.StatusOK, ""},
		{"execute select in a view with custom join clause", "/%s/public/view_test?_join=inner:test2:test2.name:eq:view_test.player", "GET", http.StatusOK, ""},
		{"execute select in a view with custom where clause and pagination", "/%s/public/view_test?player=$eq.gopher&_page=1&_page_size=20", "GET", http.StatusOK, ""},
		{"execute select in a view with select fields", "/%s/public/view_test?_select=player", "GET", http.StatusOK, ""},

		{"execute select in a table with invalid join clause", "/%s/public/test?_join=inner:test2:test2.name", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid where clause", "/%s/public/test?0name=$eq.test", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with order clause and column invalid", "/%s/public/test?_order=0name", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid pagination clause", "/%s/public/test?name=$eq.test&_page=A", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid where clause", "/%s/public/test?0name=$eq.test", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid count clause", "/%s/public/test?_count=0name", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid order clause", "/%s/public/test?_order=0name", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid fields using group by clause", "/%s/public/test_group_by_table?_select=pa,sum:pum&_groupby=pa", "GET", http.StatusBadRequest, ""},
		{"execute select in a table with invalid fields using group by and having clause", "/%s/public/test_group_by_table?_select=pa,sum:pum&_groupby=pa->>having:sum:pmu:$eq:150", "GET", http.StatusBadRequest, ""},

		{"execute select in a view with an other column", "/%s/public/view_test?_select=celphone", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with where and column invalid", "/%s/public/view_test?0celphone=$eq.888888", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with custom join clause invalid", "/%s/public/view_test?_join=inner:test2.name:eq:view_test.player", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with custom where clause and pagination invalid", "/%s/public/view_test?player=$eq.gopher&_page=A&_page_size=20", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with order by and column invalid", "/%s/public/view_test?_order=0celphone", "GET", http.StatusBadRequest, ""},
		{"execute select in a view with count column invalid", "/%s/public/view_test?_count=0celphone", "GET", http.StatusBadRequest, ""},

		{"execute select in a db that does not exist", "/invalid/public/view_test?_count=0celphone", "GET", http.StatusBadRequest, ""},
	}
	for _, db := range databases {
		config.PrestConf.SingleDB = false

		for _, tc := range testCases {
			t.Log(fmt.Sprintf("(DB: %s) %s", db, tc.description))
			//config.PrestConf = &config.Prest{}
			//config.Load()

			if tc.body != "" {
				testutils.DoRequest(t, fmt.Sprintf(server.URL+tc.url, db), nil, tc.method, tc.status, "SelectFromTables", tc.body)
				continue
			}
			testutils.DoRequest(t, fmt.Sprintf(server.URL+tc.url, db), nil, tc.method, tc.status, "SelectFromTables")
		}
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

	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(InsertInTables)).
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

	router := mux.NewRouter()
	router.HandleFunc("/batch/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(BatchInsertInTables)).
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
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(DeleteFromTable)).
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
	router := mux.NewRouter()
	router.HandleFunc("/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(UpdateTable)).
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
	router := mux.NewRouter()
	router.HandleFunc("/show/{database}/{schema}/{table}", setHTTPTimeoutMiddleware(ShowTable)).
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
