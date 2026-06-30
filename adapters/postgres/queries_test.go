package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func TestGetScript_InvalidVerb(t *testing.T) {
	withPrestConf(t, defaultTestConf())
	adapter := testAdapter()

	_, err := adapter.GetScript("ANY", "folder", "script")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid http method")
}

func TestGetScript_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg := defaultTestConf()
	cfg.QueriesPath = dir
	withPrestConf(t, cfg)
	adapter := testAdapter()

	_, err := adapter.GetScript("GET", "missing", "script")
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not load script")
}

func TestGetScript_Success(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "queries")
	require.NoError(t, os.MkdirAll(folder, 0o755))
	scriptPath := filepath.Join(folder, "list.read.sql")
	require.NoError(t, os.WriteFile(scriptPath, []byte("SELECT 1"), 0o644))

	cfg := defaultTestConf()
	cfg.QueriesPath = dir
	withPrestConf(t, cfg)
	adapter := testAdapter()

	got, err := adapter.GetScript("GET", "queries", "list")
	require.NoError(t, err)
	require.Equal(t, scriptPath, got)
}

func TestParseScript_Template(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "query.read.sql")
	require.NoError(t, os.WriteFile(scriptPath, []byte(`SELECT * FROM users WHERE name = '{{ .field1 }}'`), 0o644))

	withPrestConf(t, defaultTestConf())
	adapter := testAdapter()

	sql, values, err := adapter.ParseScript(scriptPath, map[string]interface{}{"field1": "abc"})
	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM users WHERE name = 'abc'", sql)
	require.Empty(t, values)
}

func TestParseScript_InvalidTemplate(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "bad.read.sql")
	require.NoError(t, os.WriteFile(scriptPath, []byte(`{{ .missing`), 0o644))

	withPrestConf(t, defaultTestConf())
	adapter := testAdapter()

	_, _, err := adapter.ParseScript(scriptPath, map[string]interface{}{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not parse file")
}

func TestExecuteScripts_InvalidMethod(t *testing.T) {
	withPrestConf(t, defaultTestConf())
	adapter := testAdapter()

	sc := adapter.ExecuteScripts("ANY", "SELECT 1", nil)
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "invalid method")
	require.Empty(t, sc.Bytes())
}

func TestExecuteScripts_GET(t *testing.T) {
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
	_, mock := withSQLMock(t)

	mock.ExpectPrepare(`UPDATE users`).
		ExpectExec().
		WillReturnResult(sqlmock.NewResult(0, 2))

	sc := WriteSQL("UPDATE users SET active=true", nil)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":2}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWriteSQL_PrepareError(t *testing.T) {
	_, mock := withSQLMock(t)

	mock.ExpectPrepare(`DELETE FROM users`).WillReturnError(errors.New("prepare failed"))

	sc := WriteSQL("DELETE FROM users", nil)
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "could not prepare sql")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteScriptsCtx_WithContext(t *testing.T) {
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
	_, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`DELETE FROM users`).
		ExpectExec().
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := WriteSQLCtx(ctx, "DELETE FROM users WHERE id=1", nil)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}
