package controllers

import (
	"errors"
	"fmt"
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

// TestScriptHandler_Execute_RejectsInterpolatedRejectedParam is the fix for the
// silent-failure half of issue #1030: a parameter the screen refuses used to be
// blanked, leaving the query to run with ” and return rows for a different
// record under HTTP 200. Interpolating it must now fail the request outright.
func TestScriptHandler_Execute_RejectsInterpolatedRejectedParam(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().
		ResolveScript(gomock.Any(), http.MethodGet, "queries", "list", "prest-test").
		Return(adapters.ScriptSource{Name: "list.read.sql", Content: `SELECT '{{.slug}}'`}, nil)
	// The template renders the marker, which is what makes the request fail. The
	// executor must never be reached.
	scripts.EXPECT().
		ParseScriptTemplate("list.read.sql", `SELECT '{{.slug}}'`, gomock.Any()).
		DoAndReturn(func(_, _ string, data map[string]interface{}) (string, []interface{}, error) {
			return fmt.Sprintf("SELECT '%v'", data["slug"]), nil, nil
		})

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("prest-test").Return(true)

	h := NewScriptHandler(Deps{
		Scripts:  scripts,
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		DB:       db,
	})
	req := httptest.NewRequest(http.MethodGet, "/prest-test/queries/list?slug=compra+do+mes", nil)
	req = mux.SetURLVars(req, map[string]string{"database": "prest-test", "queriesLocation": "queries", "script": "list"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "slug")
	require.NotContains(t, rec.Body.String(), "compra do mes")
}

// A value that reaches sqlVal instead of being interpolated is bound by the
// driver, so the screen has nothing to protect against and the request succeeds.
func TestScriptHandler_Execute_AllowsRejectedParamWhenOnlyBound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scripts := mockgen.NewMockScriptRunner(ctrl)
	scripts.EXPECT().
		ResolveScript(gomock.Any(), http.MethodGet, "queries", "list", "prest-test").
		Return(adapters.ScriptSource{Name: "list.read.sql", Content: `SELECT {{sqlVal "slug"}}`}, nil)
	// Stands in for the real registry: binds the raw value, never renders the marker.
	scripts.EXPECT().
		ParseScriptTemplate("list.read.sql", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_, _ string, data map[string]interface{}) (string, []interface{}, error) {
			raw := data[rawParamKey].(map[string]interface{})
			return "SELECT $1", []interface{}{raw["slug"]}, nil
		})

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"?column?":"compra do mes"}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().
		ExecuteScriptsCtx(gomock.Any(), http.MethodGet, "SELECT $1", []interface{}{"compra do mes"}).
		Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("prest-test").Return(true)

	h := NewScriptHandler(Deps{Scripts: scripts, Executor: executor, DB: db})
	req := httptest.NewRequest(http.MethodGet, "/prest-test/queries/list?slug=compra+do+mes", nil)
	req = mux.SetURLVars(req, map[string]string{"database": "prest-test", "queriesLocation": "queries", "script": "list"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()

	h.Execute(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "compra do mes")
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

func TestExtractHeaders_DropsCredentialHeaders(t *testing.T) {
	t.Parallel()

	// A JWT passes sanitizeScriptParam's allow-list unchanged (base64url, dots
	// and the space in "Bearer " are all permitted), so without an explicit
	// drop a template referencing {{.header.Authorization}} would interpolate
	// the caller's token into the SQL text — which is logged at debug level.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJnb3BoZXIifQ.c2ln")
	req.Header.Set("Cookie", "session=abc123")
	req.Header.Set("Proxy-Authorization", "Basic Z29waGVyOnMzY3JldA")
	req.Header.Set("X-Api-Key", "k3y")
	req.Header.Set("X-Application", "prest")

	data := map[string]interface{}{}
	extractHeaders(req, data)

	headers := data["header"].(map[string]interface{})
	// The key stays so existing templates still render an empty literal, but it
	// never carries the credential.
	require.Equal(t, "", headers["Authorization"])
	require.Equal(t, "", headers["Cookie"])
	require.Equal(t, "", headers["Proxy-Authorization"])
	require.Equal(t, "", headers["X-Api-Key"])
	// Non-credential headers keep working.
	require.Equal(t, "prest", headers["X-Application"])
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

// The character allow-list permits letters, digits and space, which is enough
// to compose a read-only injection when a template interpolates the value in an
// unquoted context (`WHERE id = {{.id}}`) — no quote, comma or parenthesis
// required. These payloads must be blanked, not returned verbatim.
func TestSanitizeScriptParam_RejectsUnquotedContextInjection(t *testing.T) {
	t.Parallel()

	unsafe := []string{
		"0 OR true",
		"0 UNION SELECT users::text FROM users",
		"0 union select passwd from pg_shadow",
		"0 UNION SELECT table_name FROM information_schema.tables",
		"0 UNION SELECT query FROM pg_stat_activity",
		"1 -- comment",
		"1::text",
		"0 Union Select 1",
		// PostgreSQL before v15 accepts `0union` as `0` + keyword, so a token's
		// numeric prefix must not hide the keyword behind it.
		"0union select 1",
	}
	for _, value := range unsafe {
		require.Equal(t, "", sanitizeScriptParam(value), "value must be rejected: %s", value)
	}

	// Values that carry no SQL keyword or comment/cast token stay untouched, so
	// existing quoted-context templates keep working.
	// A keyword is only a keyword as a whole token: identifiers that merely
	// start with one (order66, select2, table9) must survive.
	safe := []string{"gopher", "42", "2024-01-01", "user@example.com", "John Doe", "a/b.c",
		"order66", "select2", "table9", "union_member"}
	for _, value := range safe {
		require.Equal(t, value, sanitizeScriptParam(value), "value must be preserved: %s", value)
	}
}

// Composing SQL in an unquoted context requires separating tokens, and space is
// the only separator the character allow-list permits. A single-token value can
// therefore carry a keyword harmlessly — `WHERE 1 = teste-do-abc` is a syntax
// error, not an injection — so screening it as SQL only blanks legitimate data.
// Portuguese slugs are the reported casualty: `do` appears in a large share of
// them (issue #1030).
func TestSanitizeScriptParam_PreservesSingleTokenValues(t *testing.T) {
	t.Parallel()

	safe := []string{
		"teste-do-abc",
		"compra-do-mes",
		"estatua-do-diabo",
		"significado-do-yom-kippur",
		"trombetas-do-apocalipse",
		"teste-select-abc",
		"teste-as-abc",
		// Whole-token keywords with no separator at all must survive too, since
		// they still cannot compose anything on their own.
		"do",
		"select",
	}
	for _, value := range safe {
		require.Equal(t, value, sanitizeScriptParam(value), "value must be preserved: %s", value)
	}
}

// A rejected value renders empty, exactly as before, but reports itself so the
// request can fail rather than return rows for a different record. Rendering is
// what triggers it: a value that only ever reaches sqlVal is never interpolated
// and must not fail anything.
func TestExtractQueryParameters_RejectedValueFailsOnlyWhenInterpolated(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.RawQuery = "safe=ok&unsafe=%27%3BDROP"

	data := map[string]interface{}{}
	rejected := extractQueryParameters(req, data)

	// Nothing rendered yet, so nothing to report.
	require.NoError(t, rejected.err())

	// Interpolating it is what text/template does via fmt.Stringer.
	require.Equal(t, "", fmt.Sprint(data["unsafe"]))

	err := rejected.err()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsafe")
	require.NotContains(t, err.Error(), "DROP", "the rejected value must never be echoed back")
}

func TestExtractQueryParameters_RejectsUnsafeValueInMultiValueParam(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.RawQuery = "tag=good&tag=bad%27%3B--"

	data := map[string]interface{}{}
	rejected := extractQueryParameters(req, data)

	values := data["tag"].([]interface{})
	require.Equal(t, "good", fmt.Sprint(values[0]))
	require.Equal(t, "", fmt.Sprint(values[1]))

	err := rejected.err()
	require.Error(t, err)
	require.Contains(t, err.Error(), "tag")
}

func TestExtractQueryParameters_KeepsSafeValues(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.RawQuery = "safe=ok&slug=teste-do-abc&tag=good&tag=fine"

	data := map[string]interface{}{}
	rejected := extractQueryParameters(req, data)

	require.Equal(t, "ok", data["safe"])
	require.Equal(t, "teste-do-abc", data["slug"])
	// Stays a []string so inFormat and sqlList keep working.
	require.Equal(t, []string{"good", "fine"}, data["tag"])
	require.NoError(t, rejected.err())
}

// Values reach the binding helpers unscreened: sqlVal passes them to the driver
// as a bound parameter, where SQL composition is impossible by construction.
func TestExtractQueryParameters_ExposesRawValues(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.RawQuery = "phrase=compra+do+mes&tag=a+or+b&tag=plain"

	data := map[string]interface{}{}
	require.NoError(t, extractQueryParameters(req, data).err())

	raw := data[rawParamKey].(map[string]interface{})
	require.Equal(t, "compra do mes", raw["phrase"])
	require.Equal(t, []string{"a or b", "plain"}, raw["tag"])

	// The inline value is still screened, so un-migrated templates are unchanged:
	// it renders empty (and, being rejected, would then fail the request).
	require.Equal(t, "", fmt.Sprint(data["phrase"]))
}

// Credential blanking is about secrecy, not SQL composition, so binding must not
// become a way around it.
func TestExtractHeaders_RawMapStillBlanksCredentials(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJnb3BoZXIifQ.c2ln")
	req.Header.Set("X-Application", "compra do mes")

	data := map[string]interface{}{}
	extractHeaders(req, data)

	raw := data[rawHeaderKey].(map[string]interface{})
	require.Equal(t, "", raw["Authorization"])
	// A non-credential header keeps its unscreened value for binding.
	require.Equal(t, "compra do mes", raw["X-Application"])
}
