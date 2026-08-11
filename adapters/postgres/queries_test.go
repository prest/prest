package postgres

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prest/prest/v2/adapters"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func TestGetScript_InvalidVerb(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	_, err := adapter.GetScript("ANY", "folder", "script")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid http method")
}

func TestGetScript_MissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg := defaultTestConf()
	cfg.QueriesPath = dir
	adapter := testAdapter(cfg)

	_, err := adapter.GetScript("GET", "missing", "script")
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not load script")
}

func TestGetScript_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	folder := filepath.Join(dir, "queries")
	require.NoError(t, os.MkdirAll(folder, 0o755))
	scriptPath := filepath.Join(folder, "list.read.sql")
	require.NoError(t, os.WriteFile(scriptPath, []byte("SELECT 1"), 0o644))

	cfg := defaultTestConf()
	cfg.QueriesPath = dir
	adapter := testAdapter(cfg)

	got, err := adapter.GetScript("GET", "queries", "list")
	require.NoError(t, err)
	require.Equal(t, scriptPath, got)
}

// TestGetScript_RejectsTraversal keeps the path barrier inside the adapter
// rather than relying on the controller's validatePathSegments gate: getScriptPath
// joins caller-supplied folder and name onto the queries directory, so any caller
// that skips that gate could otherwise read arbitrary files (CodeQL go/path-injection).
func TestGetScript_RejectsTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// A readable file one level above the queries directory: the target an
	// escaping path would reach if the join were unchecked.
	outside := filepath.Join(dir, "secret.read.sql")
	require.NoError(t, os.WriteFile(outside, []byte("SELECT 'pwned'"), 0o644))

	queries := filepath.Join(dir, "queries")
	require.NoError(t, os.MkdirAll(queries, 0o755))

	cfg := defaultTestConf()
	cfg.QueriesPath = queries
	adapter := testAdapter(cfg)

	var testCases = []struct {
		description string
		folder      string
		script      string
	}{
		{"parent traversal in the folder segment", "..", "secret"},
		{"parent traversal in the script segment", ".", "../secret"},
		{"nested traversal that normalizes above the base", "sub/../..", "secret"},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		_, err := adapter.GetScript("GET", tc.folder, tc.script)
		require.Error(t, err, tc.description)
		require.Contains(t, err.Error(), "invalid script path", tc.description)
	}

	// An absolute folder segment is not an escape: filepath.Join drops the leading
	// separator, re-rooting it under the queries directory. It fails as a missing
	// file rather than as a rejected path, and must not reach the outside file.
	_, err := adapter.GetScript("GET", dir, "secret")
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not load script")
}

// TestGetScript_RejectsSymlinkEscape covers what the lexical containment check
// cannot see: a symlink that sits inside the queries directory but resolves to a
// file outside it. The joined path looks contained, so only resolving the link
// catches the escape.
func TestGetScript_RejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outside := filepath.Join(dir, "secret.read.sql")
	require.NoError(t, os.WriteFile(outside, []byte("SELECT 'pwned'"), 0o644))

	queries := filepath.Join(dir, "queries", "fulltable")
	require.NoError(t, os.MkdirAll(queries, 0o755))
	// escape.read.sql lives inside the queries tree but points above it.
	require.NoError(t, os.Symlink(outside, filepath.Join(queries, "escape.read.sql")))

	cfg := defaultTestConf()
	cfg.QueriesPath = filepath.Join(dir, "queries")
	adapter := testAdapter(cfg)

	_, err := adapter.GetScript("GET", "fulltable", "escape")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid script path")
}

// TestGetScript_AllowsSymlinkWithinBase confirms the symlink resolution does not
// break the legitimate case of a link that stays inside the queries directory,
// which operators use to share one script across folders.
func TestGetScript_AllowsSymlinkWithinBase(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	shared := filepath.Join(base, "shared")
	require.NoError(t, os.MkdirAll(shared, 0o755))
	target := filepath.Join(shared, "list.read.sql")
	require.NoError(t, os.WriteFile(target, []byte("SELECT 1"), 0o644))

	folder := filepath.Join(base, "fulltable")
	require.NoError(t, os.MkdirAll(folder, 0o755))
	link := filepath.Join(folder, "list.read.sql")
	require.NoError(t, os.Symlink(target, link))

	cfg := defaultTestConf()
	cfg.QueriesPath = base
	adapter := testAdapter(cfg)

	got, err := adapter.GetScript("GET", "fulltable", "list")
	require.NoError(t, err)
	require.Equal(t, link, got)
}

func TestParseScript_Template(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "query.read.sql")
	require.NoError(t, os.WriteFile(scriptPath, []byte(`SELECT * FROM users WHERE name = '{{ .field1 }}'`), 0o644))

	adapter := testAdapter()

	sql, values, err := adapter.ParseScript(scriptPath, map[string]interface{}{"field1": "abc"})
	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM users WHERE name = 'abc'", sql)
	require.Empty(t, values)
}

func TestParseScript_InvalidTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "bad.read.sql")
	require.NoError(t, os.WriteFile(scriptPath, []byte(`{{ .missing`), 0o644))

	adapter := testAdapter()

	_, _, err := adapter.ParseScript(scriptPath, map[string]interface{}{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not parse template")
}

func TestExecuteScripts_InvalidMethod(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	sc := adapter.ExecuteScripts("ANY", "SELECT 1", nil)
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "invalid method")
	require.Empty(t, sc.Bytes())
}

func TestExecuteScripts_GET(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT \* FROM users\) s`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[{"id":1}]`)))

	sc := adapter.ExecuteScripts("GET", "SELECT * FROM users", nil)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"id":1}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteScripts_POST(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`INSERT INTO users`).
		ExpectExec().
		WillReturnResult(sqlmock.NewResult(1, 1))

	sc := adapter.ExecuteScripts("POST", "INSERT INTO users(name) VALUES('alice')", nil)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWriteSQL_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`UPDATE users`).
		ExpectExec().
		WillReturnResult(sqlmock.NewResult(0, 2))

	sc := adapter.WriteSQL("UPDATE users SET active=true", nil)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":2}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWriteSQL_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`DELETE FROM users`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.WriteSQL("DELETE FROM users", nil)
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "could not prepare sql")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteScriptsCtx_WithContext(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT 1\) s`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))

	sc := adapter.ExecuteScriptsCtx(ctx, "GET", "SELECT 1", nil)
	require.NoError(t, sc.Err())
	require.Equal(t, "[1]", string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestWriteSQLCtx_Success(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`DELETE FROM users`).
		ExpectExec().
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.WriteSQLCtx(ctx, "DELETE FROM users WHERE id=1", nil)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestWriteSQLCtx_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`DELETE FROM users`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.WriteSQLCtx(ctx, "DELETE FROM users", nil)
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "could not prepare sql")
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestWriteSQLCtx_ExecError(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`DELETE FROM users`).
		ExpectExec().
		WillReturnError(errors.New("exec failed"))

	sc := adapter.WriteSQLCtx(ctx, "DELETE FROM users", nil)
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "could not peform sql")
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestExecuteScriptsCtx_WriteMethods(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)

	testCases := []struct {
		method      string
		sql         string
		prepareLike string
	}{
		{"POST", "INSERT INTO users(name) VALUES('alice')", `INSERT INTO users`},
		{"PUT", "UPDATE users SET active=true", `UPDATE users SET`},
		{"PATCH", "UPDATE users SET active=false WHERE id=1", `UPDATE users SET`},
		{"DELETE", "DELETE FROM users WHERE id=1", `DELETE FROM users`},
	}

	for _, tc := range testCases {
		t.Run(tc.method, func(t *testing.T) {
			ctxMock.ExpectPrepare(tc.prepareLike).
				ExpectExec().
				WillReturnResult(sqlmock.NewResult(0, 1))

			sc := adapter.ExecuteScriptsCtx(ctx, tc.method, tc.sql, nil)
			require.NoError(t, sc.Err())
			require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
		})
	}
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

// captureDebugLogs redirects the default slog logger into a buffer at debug level
// so a test can assert on what was — and was not — logged. It swaps a
// process-wide global, so callers must not run in parallel.
func captureDebugLogs(t *testing.T) func() string {
	t.Helper()

	buf := &syncBuffer{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return buf.String
}

// syncBuffer is a bytes.Buffer safe for the driver to write to from any goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestWriteSQLCtx_ErrorsDoNotLogStatement covers the script-reachable write verbs.
// POST, PUT, PATCH and DELETE scripts all execute through WriteSQLCtx, and a
// script template renders request headers and query parameters straight into the
// statement — so the SQL text on this path is caller-controlled data, not a fixed
// pREST-built statement. Neither the prepare nor the exec failure may put it in
// the logs, at any level (CodeQL go/clear-text-logging).
//
// Not parallel: captureDebugLogs replaces the global default logger.
func TestWriteSQLCtx_ErrorsDoNotLogStatement(t *testing.T) {
	logs := captureDebugLogs(t)

	adapter, defaultMock, ctxMock := withSQLMocks(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)

	// A marker that could only appear in the logs by way of the statement itself.
	const marker = "SCRIPT_RENDERED_SECRET"

	testCases := []struct {
		description string
		method      string
		sql         string
		prepareLike string
	}{
		{"POST script failing to prepare", "POST",
			"INSERT INTO users(note) VALUES('" + marker + "')", `INSERT INTO users`},
		{"PUT script failing to prepare", "PUT",
			"UPDATE users SET note='" + marker + "'", `UPDATE users SET`},
		{"PATCH script failing to prepare", "PATCH",
			"UPDATE users SET note='" + marker + "' WHERE id=1", `UPDATE users SET`},
		{"DELETE script failing to prepare", "DELETE",
			"DELETE FROM users WHERE note='" + marker + "'", `DELETE FROM users`},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		ctxMock.ExpectPrepare(tc.prepareLike).WillReturnError(errors.New("prepare failed"))

		sc := adapter.ExecuteScriptsCtx(ctx, tc.method, tc.sql, nil)
		require.Error(t, sc.Err(), tc.description)
	}

	// Exec failure takes a different branch and must be just as quiet.
	t.Log("DELETE script failing to execute")
	ctxMock.ExpectPrepare(`DELETE FROM users`).
		ExpectExec().
		WillReturnError(errors.New("exec failed"))
	sc := adapter.ExecuteScriptsCtx(ctx, "DELETE",
		"DELETE FROM users WHERE note='"+marker+"'", nil)
	require.Error(t, sc.Err())

	require.NotContains(t, logs(), marker,
		"a script-rendered statement must never reach the logs")
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

// TestWriteSQL_ErrorsDoNotLogStatement is the same contract for the non-context
// entry point, which ExecuteScripts (the legacy script path) still uses.
//
// Not parallel: captureDebugLogs replaces the global default logger.
func TestWriteSQL_ErrorsDoNotLogStatement(t *testing.T) {
	logs := captureDebugLogs(t)

	adapter, mock := withSQLMock(t)
	const marker = "SCRIPT_RENDERED_SECRET"

	// Prepare failure.
	mock.ExpectPrepare(`DELETE FROM users`).WillReturnError(errors.New("prepare failed"))
	sc := adapter.ExecuteScripts("DELETE", "DELETE FROM users WHERE note='"+marker+"'", nil)
	require.Error(t, sc.Err())

	// Exec failure.
	mock.ExpectPrepare(`INSERT INTO users`).
		ExpectExec().
		WillReturnError(errors.New("exec failed"))
	sc = adapter.ExecuteScripts("POST", "INSERT INTO users(note) VALUES('"+marker+"')", nil)
	require.Error(t, sc.Err())

	require.NotContains(t, logs(), marker,
		"a script-rendered statement must never reach the logs")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestCRUDBuilders_StillLogStatementAtDebug is the other half of the contract:
// the statement is still available to operators on the paths a script cannot
// reach. The CRUD insert/update/delete builders compose their SQL from validated
// identifiers with bound parameters, so logging it at debug level costs nothing
// and is the main way a failing query gets diagnosed.
//
// Not parallel: captureDebugLogs replaces the global default logger.
func TestCRUDBuilders_StillLogStatementAtDebug(t *testing.T) {
	logs := captureDebugLogs(t)

	adapter, mock := withSQLMock(t)

	// Only update and delete report a prepare failure through logFailedSQL. Insert
	// routes its prepare through fullInsert, which reports the error without the
	// statement, so there is no logFailedSQL call on that path to assert on.
	testCases := []struct {
		description string
		sql         string
		prepareLike string
		call        func(string) adapters.Scanner
	}{
		{"update builder", `UPDATE public.users SET name=$1`,
			`UPDATE`, func(s string) adapters.Scanner { return adapter.Update(s) }},
		{"delete builder", `DELETE FROM public.users WHERE id=$1`,
			`DELETE FROM`, func(s string) adapters.Scanner { return adapter.Delete(s) }},
	}

	for _, tc := range testCases {
		t.Log(tc.description)
		mock.ExpectPrepare(tc.prepareLike).WillReturnError(errors.New("prepare failed"))

		sc := tc.call(tc.sql)
		require.Error(t, sc.Err(), tc.description)
	}

	// Each statement reached the debug log through logFailedSQL — these paths are
	// unreachable from a script template, so the SQL is pREST-built and safe to log.
	out := logs()
	require.Contains(t, out, "failed sql")
	for _, tc := range testCases {
		require.Contains(t, out, tc.sql, tc.description)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
