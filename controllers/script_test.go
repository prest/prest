package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/middlewares"
	"github.com/stretchr/testify/require"
)

func TestScriptHandler_Execute_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().ResolveScript(gomock.Any(), http.MethodGet, "queries", "list", "prest-test").Return(adapters.ScriptSource{
		Name: "list.read.sql", Content: "SELECT 1",
	}, nil)
	scripts.EXPECT().ParseScriptTemplate("list.read.sql", "SELECT 1", gomock.Any()).Return(`SELECT 1`, nil, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"n":1}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ExecuteScriptsCtx(gomock.Any(), http.MethodGet, `SELECT 1`, gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("prest-test").Return(true)

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
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().ResolveScript(gomock.Any(), http.MethodGet, "queries", "ping", "").Return(adapters.ScriptSource{
		Name: "ping.read.sql", Content: "SELECT 1",
	}, nil)
	scripts.EXPECT().ParseScriptTemplate(gomock.Any(), gomock.Any(), gomock.Any()).Return(`SELECT 1`, nil, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ExecuteScriptsCtx(gomock.Any(), http.MethodGet, `SELECT 1`, gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()
	db.EXPECT().IsRegistered("prest-test").Return(true)

	h := NewScriptHandler(Deps{Scripts: scripts, Executor: executor, DB: db, PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodGet, "/queries/ping", nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "queries", "script": "ping"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestScriptHandler_Execute_WithCache(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().ResolveScript(gomock.Any(), http.MethodGet, "queries", "list", "prest-test").Return(adapters.ScriptSource{
		Name: "list.read.sql", Content: "SELECT 1",
	}, nil)
	scripts.EXPECT().ParseScriptTemplate(gomock.Any(), gomock.Any(), gomock.Any()).Return(`SELECT 1`, nil, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`cached`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ExecuteScriptsCtx(gomock.Any(), http.MethodGet, `SELECT 1`, gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("prest-test").Return(true)
	cacher := &recordingCacher{}
	h := NewScriptHandler(Deps{Scripts: scripts, Executor: executor, DB: db, PGDatabase: "prest-test", Cache: cacher})

	url := "/queries/list?x=1"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "queries", "script": "list", "database": "prest-test"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, middlewares.CacheKey(req), cacher.key)
	require.Equal(t, "cached", cacher.value)
}

// TestScriptHandler_Execute_RejectsUnregisteredDatabase guards against an
// unauthenticated request reaching the connection layer with an arbitrary,
// attacker-chosen database name. Unlike CRUDHandler.Select and every other
// controller, ScriptHandler.Execute used to skip validateDatabase entirely,
// so dbFromCtx would attempt a real outbound Postgres connection (via
// AddDatabaseToPool) for any name in the /_QUERIES/{database}/... path.
// ResolveScript must never be reached once the database fails validation.
func TestScriptHandler_Execute_RejectsUnregisteredDatabase(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("evil").Return(false)

	h := NewScriptHandler(Deps{
		Scripts:  mockgen.NewMockScriptRunner(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		DB:       db,
	})
	req := httptest.NewRequest(http.MethodGet, "/evil/queries/list", nil)
	req = mux.SetURLVars(req, map[string]string{"database": "evil", "queriesLocation": "queries", "script": "list"})
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestScriptHandler_Execute_RejectsUnsafePathSegment guards against path
// segments (queriesLocation/script) that fall outside the safe identifier
// charset ScriptHandler relies on to build the on-disk template path.
func TestScriptHandler_Execute_RejectsUnsafePathSegment(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("prest-test").Return(true)

	h := NewScriptHandler(Deps{
		Scripts:  mockgen.NewMockScriptRunner(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		DB:       db,
	})
	req := httptest.NewRequest(http.MethodGet, "/prest-test/queries/../../etc", nil)
	req = mux.SetURLVars(req, map[string]string{"database": "prest-test", "queriesLocation": "queries", "script": "../../etc"})
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestScriptHandler_ExecuteScriptQuery_GetScriptError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().ResolveScript(gomock.Any(), http.MethodGet, "queries", "missing", "").Return(adapters.ScriptSource{}, errors.New("not found"))

	db := mockgen.NewMockDatabaseRegistry(ctrl)

	h := NewScriptHandler(Deps{Scripts: scripts, Executor: mockgen.NewMockQueryExecutor(ctrl), DB: db, PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodGet, "/queries/missing", nil)

	_, err := h.ExecuteScriptQuery(req, "queries", "missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not get script")
}

func TestScriptHandler_ExecuteScriptQuery_ExecuteError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().ResolveScript(gomock.Any(), http.MethodGet, "queries", "bad", "").Return(adapters.ScriptSource{
		Name: "bad.read.sql", Content: "SELECT bad",
	}, nil)
	scripts.EXPECT().ParseScriptTemplate("bad.read.sql", "SELECT bad", gomock.Any()).Return(`SELECT bad`, nil, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("syntax error"))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ExecuteScriptsCtx(gomock.Any(), http.MethodGet, `SELECT bad`, gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)

	h := NewScriptHandler(Deps{Scripts: scripts, Executor: executor, DB: db, PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodGet, "/queries/bad", nil)
	req = req.WithContext(withTestTimeout(req.Context()))

	_, err := h.ExecuteScriptQuery(req, "queries", "bad")
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not execute sql")
}

func TestExtractHeaders(t *testing.T) {
	t.Parallel()

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

func TestExtractHeaders_SanitizesUnsafeValues(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Safe", "prest")
	req.Header.Set("X-Injection", "' UNION SELECT rolpassword::text FROM pg_authid WHERE rolname='postgres' -- ")
	req.Header.Add("X-Multi", "good")
	req.Header.Add("X-Multi", "'; DROP TABLE users; --")

	data := map[string]interface{}{}
	extractHeaders(req, data)

	headers := data["header"].(map[string]interface{})
	require.Equal(t, "prest", headers["X-Safe"])
	require.Equal(t, "", headers["X-Injection"])
	require.Equal(t, []string{"good", ""}, headers["X-Multi"])
}

func TestExtractQueryParameters(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/?foo=bar&tag=a&tag=b", nil)

	data := map[string]interface{}{}
	extractQueryParameters(req, data)

	require.Equal(t, "bar", data["foo"])
	require.Equal(t, []string{"a", "b"}, data["tag"])
}

func TestSanitizeScriptParam(t *testing.T) {
	t.Parallel()

	require.Equal(t, "abc123", sanitizeScriptParam("abc123"))
	require.Equal(t, "foo_bar-baz", sanitizeScriptParam("foo_bar-baz"))
	require.Equal(t, "user@example.com", sanitizeScriptParam("user@example.com"))
	require.Equal(t, "", sanitizeScriptParam("'; DROP TABLE users; --"))
	require.Equal(t, "", sanitizeScriptParam(`" OR 1=1`))
}

func TestExtractQueryParameters_SanitizesUnsafeValues(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/?safe=ok&tag=good&tag=bad%27%3B--", nil)
	req.URL.RawQuery = "safe=ok&unsafe=%27%3BDROP&tag=good&tag=bad%27%3B--"

	data := map[string]interface{}{}
	extractQueryParameters(req, data)

	require.Equal(t, "ok", data["safe"])
	require.Equal(t, "", data["unsafe"])
	require.Equal(t, []string{"good", ""}, data["tag"])
}
