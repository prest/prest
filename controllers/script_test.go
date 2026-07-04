package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestScriptHandler_Execute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().GetScript(http.MethodGet, "queries", "list").Return("/tmp/list.sql", nil)
	scripts.EXPECT().ParseScript("/tmp/list.sql", gomock.Any()).Return(`SELECT 1`, nil, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"n":1}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ExecuteScriptsCtx(gomock.Any(), http.MethodGet, `SELECT 1`, gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().SetDatabase("prest-test")

	h := NewScriptHandler(Deps{Scripts: scripts, Executor: executor, DB: db, PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodGet, "/queries/list", nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "queries", "script": "list", "database": "prest-test"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"n":1`)
}

func TestScriptHandler_Execute_DefaultDatabase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().GetScript(http.MethodGet, "queries", "ping").Return("/tmp/ping.sql", nil)
	scripts.EXPECT().ParseScript(gomock.Any(), gomock.Any()).Return(`SELECT 1`, nil, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ExecuteScriptsCtx(gomock.Any(), http.MethodGet, `SELECT 1`, gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().SetDatabase("prest-test")
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewScriptHandler(Deps{Scripts: scripts, Executor: executor, DB: db, PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodGet, "/queries/ping", nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "queries", "script": "ping"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestScriptHandler_Execute_WithCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().GetScript(http.MethodGet, "queries", "list").Return("/tmp/list.sql", nil)
	scripts.EXPECT().ParseScript(gomock.Any(), gomock.Any()).Return(`SELECT 1`, nil, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`cached`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ExecuteScriptsCtx(gomock.Any(), http.MethodGet, `SELECT 1`, gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().SetDatabase("prest-test")

	cacher := &recordingCacher{}
	h := NewScriptHandler(Deps{Scripts: scripts, Executor: executor, DB: db, PGDatabase: "prest-test", Cache: cacher})

	url := "/queries/list?x=1"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "queries", "script": "list", "database": "prest-test"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, url, cacher.key)
	require.Equal(t, "cached", cacher.value)
}

func TestScriptHandler_ExecuteScriptQuery_GetScriptError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().GetScript(http.MethodGet, "queries", "missing").Return("", errors.New("not found"))

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().SetDatabase("prest-test")

	h := NewScriptHandler(Deps{Scripts: scripts, Executor: mockgen.NewMockQueryExecutor(ctrl), DB: db, PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodGet, "/queries/missing", nil)

	_, err := h.ExecuteScriptQuery(req, "queries", "missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not get script")
}

func TestScriptHandler_ExecuteScriptQuery_ExecuteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().GetScript(http.MethodGet, "queries", "bad").Return("/tmp/bad.sql", nil)
	scripts.EXPECT().ParseScript("/tmp/bad.sql", gomock.Any()).Return(`SELECT bad`, nil, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("syntax error"))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ExecuteScriptsCtx(gomock.Any(), http.MethodGet, `SELECT bad`, gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().SetDatabase("prest-test")

	h := NewScriptHandler(Deps{Scripts: scripts, Executor: executor, DB: db, PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodGet, "/queries/bad", nil)
	req = req.WithContext(withTestTimeout(req.Context()))

	_, err := h.ExecuteScriptQuery(req, "queries", "bad")
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not execute sql")
}

func TestExtractHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Single", "one")
	req.Header.Add("X-Multi", "a")
	req.Header.Add("X-Multi", "b")

	data := map[string]interface{}{}
	extractHeaders(req, data)

	headers := data["header"].(map[string]interface{})
	require.Equal(t, "one", headers["X-Single"])
	require.Equal(t, []string{"a", "b"}, headers["X-Multi"])
}

func TestExtractQueryParameters(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?foo=bar&tag=a&tag=b", nil)

	data := map[string]interface{}{}
	extractQueryParameters(req, data)

	require.Equal(t, "bar", data["foo"])
	require.Equal(t, []string{"a", "b"}, data["tag"])
}

func TestSanitizeScriptParam(t *testing.T) {
	require.Equal(t, "abc123", sanitizeScriptParam("abc123"))
	require.Equal(t, "foo_bar-baz", sanitizeScriptParam("foo_bar-baz"))
	require.Equal(t, "user@example.com", sanitizeScriptParam("user@example.com"))
	require.Equal(t, "", sanitizeScriptParam("'; DROP TABLE users; --"))
	require.Equal(t, "", sanitizeScriptParam(`" OR 1=1`))
}

func TestExtractQueryParameters_SanitizesUnsafeValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?safe=ok&tag=good&tag=bad%27%3B--", nil)
	req.URL.RawQuery = "safe=ok&unsafe=%27%3BDROP&tag=good&tag=bad%27%3B--"

	data := map[string]interface{}{}
	extractQueryParameters(req, data)

	require.Equal(t, "ok", data["safe"])
	require.Equal(t, "", data["unsafe"])
	require.Equal(t, []string{"good", ""}, data["tag"])
}
