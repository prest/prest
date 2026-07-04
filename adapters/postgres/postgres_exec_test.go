package postgres

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func withFailingDBConnect(t *testing.T, msg string) *postgres {
	t.Helper()
	restore := connection.SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, errors.New(msg)
	})
	t.Cleanup(restore)
	return New(defaultTestConf()).(*postgres)
}

func withSQLMock(t *testing.T) (*postgres, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	cfg := defaultTestConf()
	pg := New(cfg).(*postgres)
	pg.conn.SetDatabase(defaultMockDB)
	pg.conn.InjectDBForTest(pg.conn.GetURI(defaultMockDB), sqlxDB)
	t.Cleanup(func() { pg.conn.ResetPoolForTest() })
	pg.ClearStmt()
	t.Cleanup(pg.ClearStmt)

	return pg, mock
}

func withSQLMocks(t *testing.T) (*postgres, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()
	defaultDB, defaultMock, err := sqlmock.New()
	require.NoError(t, err)
	ctxDB, ctxMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = defaultDB.Close()
		_ = ctxDB.Close()
	})

	cfg := defaultTestConf()
	pg := New(cfg).(*postgres)
	pg.conn.SetDatabase(defaultMockDB)
	pg.conn.InjectDBForTest(pg.conn.GetURI(defaultMockDB), sqlx.NewDb(defaultDB, "sqlmock"))
	pg.conn.InjectDBForTest(pg.conn.GetURI(contextMockDB), sqlx.NewDb(ctxDB, "sqlmock"))
	t.Cleanup(func() { pg.conn.ResetPoolForTest() })
	pg.ClearStmt()
	t.Cleanup(pg.ClearStmt)

	return pg, defaultMock, ctxMock
}

func TestQuery_SuccessEmpty(t *testing.T) {
	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT 1\) s`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte{}))

	sc := adapter.Query("SELECT 1")
	require.NoError(t, sc.Err())
	require.Equal(t, "[]", string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_SuccessWithData(t *testing.T) {
	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT \* FROM users\) s`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[{"id":1}]`)))

	sc := adapter.Query("SELECT * FROM users")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"id":1}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_PrepareError(t *testing.T) {
	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.Query("SELECT 1")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "prepare failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_ScanError(t *testing.T) {
	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg`).
		ExpectQuery().
		WillReturnError(errors.New("scan failed"))

	sc := adapter.Query("SELECT 1")
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryCtx_WithDBNameKey(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT 1\) s`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))

	sc := adapter.QueryCtx(ctx, "SELECT 1")
	require.NoError(t, sc.Err())
	require.Equal(t, "[1]", string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestInsert_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).AddRow([]byte(`{"name":"alice"}`)))

	sc := adapter.Insert(sql, "alice")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"name":"alice"}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_PrepareError(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectPrepare(`INSERT INTO`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.Insert(sql, "alice")
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInsert_InvalidSQL(t *testing.T) {
	adapter, _ := withSQLMock(t)

	sc := adapter.Insert("INVALID SQL", "alice")
	require.Error(t, sc.Err())
	require.ErrorIs(t, sc.Err(), ErrNoTableName)
}

func TestDelete_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.Delete(sql, 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_PrepareError(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectPrepare(`DELETE FROM`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.Delete(sql, 1)
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDelete_ExecError(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnError(errors.New("exec failed"))

	sc := adapter.Delete(sql, 1)
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
		ExpectExec().
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.Update(sql, "bob", 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdate_PrepareError(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	mock.ExpectPrepare(`UPDATE`).WillReturnError(errors.New("prepare failed"))

	sc := adapter.Update(sql, "bob", 1)
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestShowTable_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT table_schema`).
		ExpectQuery().
		WithArgs("users", "public").
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[{"column_name":"id"}]`)))

	sc := adapter.ShowTable("public", "users")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"column_name":"id"}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_WithStatementCache(t *testing.T) {
	cfg := &config.Prest{
		PGDatabase:  defaultMockDB,
		JSONAggType: "json_agg",
		PGCache:     true,
		PGHost:      "localhost",
		PGPort:      5432,
		PGUser:      "u",
		PGSSLMode:   "disable",
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	adapter := New(cfg).(*postgres)
	adapter.conn.SetDatabase(defaultMockDB)
	adapter.conn.InjectDBForTest(adapter.conn.GetURI(defaultMockDB), sqlxDB)
	t.Cleanup(func() { adapter.conn.ResetPoolForTest() })
	adapter.ClearStmt()
	t.Cleanup(adapter.ClearStmt)
	prep := mock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT 1\) s`)
	prep.ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))
	prep.ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))

	sc := adapter.Query("SELECT 1")
	require.NoError(t, sc.Err())

	sc = adapter.Query("SELECT 1")
	require.NoError(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuery_WithStatementCachePerDatabase(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)
	adapter.getStmts().pgCache = true

	sql := `SELECT json_agg\(s\) FROM \(SELECT 1\) s`
	defaultPrep := defaultMock.ExpectPrepare(sql)
	defaultPrep.ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[1]`)))
	ctxPrep := ctxMock.ExpectPrepare(sql)
	ctxPrep.ExpectQuery().WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[2]`)))

	sc := adapter.Query("SELECT 1")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[1]`, string(sc.Bytes()))

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sc = adapter.QueryCtx(ctx, "SELECT 1")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[2]`, string(sc.Bytes()))
	require.NoError(t, defaultMock.ExpectationsWereMet())
	require.NoError(t, ctxMock.ExpectationsWereMet())
}

func TestInsertCtx_Success(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	ctxMock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).AddRow([]byte(`{"name":"alice"}`)))

	sc := adapter.InsertCtx(ctx, sql, "alice")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"name":"alice"}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestDeleteCtx_Success(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	ctxMock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.DeleteCtx(ctx, sql, 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestUpdateCtx_Success(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	ctxMock.ExpectPrepare(`UPDATE "test"."public"."users"`).
		ExpectExec().
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	sc := adapter.UpdateCtx(ctx, sql, "bob", 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestQueryCount_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`SELECT COUNT\(\*\) FROM users`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(42)))

	sc := adapter.QueryCount(`SELECT COUNT(*) FROM users`)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"count":42}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueryCountCtx_Success(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT COUNT\(\*\) FROM users`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(7)))

	sc := adapter.QueryCountCtx(ctx, `SELECT COUNT(*) FROM users`)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"count":7}`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestShowTableCtx_Success(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`SELECT json_agg\(s\) FROM \(SELECT table_schema`).
		ExpectQuery().
		WithArgs("users", "public").
		WillReturnRows(sqlmock.NewRows([]string{"json_agg"}).AddRow([]byte(`[{"column_name":"id"}]`)))

	sc := adapter.ShowTableCtx(ctx, "public", "users")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"column_name":"id"}]`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestGetTransaction_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	mock.ExpectBegin()
	tx, err := adapter.GetTransaction()
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetTransactionCtx_Success(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectBegin()
	tx, err := adapter.GetTransactionCtx(ctx)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestInsertWithTransaction_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `INSERT INTO "test"."public"."users"("name") VALUES($1)`
	mock.ExpectBegin()
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("alice").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).AddRow([]byte(`{"name":"alice"}`)))

	tx, err := adapter.GetTransaction()
	require.NoError(t, err)
	sc := adapter.InsertWithTransaction(tx, sql, "alice")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"name":"alice"}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteWithTransaction_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `DELETE FROM "test"."public"."users" WHERE "id"=$1`
	mock.ExpectBegin()
	mock.ExpectPrepare(`DELETE FROM`).
		ExpectExec().
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := adapter.GetTransaction()
	require.NoError(t, err)
	sc := adapter.DeleteWithTransaction(tx, sql, 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateWithTransaction_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `UPDATE "test"."public"."users" SET "name"=$1 WHERE "id"=$2`
	mock.ExpectBegin()
	mock.ExpectPrepare(`UPDATE "test"."public"."users"`).
		ExpectExec().
		WithArgs("bob", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := adapter.GetTransaction()
	require.NoError(t, err)
	sc := adapter.UpdateWithTransaction(tx, sql, "bob", 1)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsertValues_Success(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sql := `INSERT INTO "test"."public"."users"("name","age") VALUES($1,$2),($3,$4)`
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("a", 1, "b", 2).
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).
			AddRow([]byte(`{"name":"a"}`)).
			AddRow([]byte(`{"name":"b"}`)))

	sc := adapter.BatchInsertValues(sql, "a", 1, "b", 2)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"name":"a"},{"name":"b"}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsertValuesCtx_Success(t *testing.T) {
	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `INSERT INTO "test"."public"."users"("name","age") VALUES($1,$2),($3,$4)`
	ctxMock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("a", 1, "b", 2).
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).
			AddRow([]byte(`{"name":"a"}`)).
			AddRow([]byte(`{"name":"b"}`)))

	sc := adapter.BatchInsertValuesCtx(ctx, sql, "a", 1, "b", 2)
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"name":"a"},{"name":"b"}]`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestBatchInsertCopy_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")

	sc := adapter.BatchInsertCopy(defaultMockDB, "public", "users", []string{"name"}, "alice")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "connect")
}

func TestBatchInsertCopyCtx_ConnectionError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)

	sc := adapter.BatchInsertCopyCtx(ctx, contextMockDB, "public", "users", []string{"name"}, "alice")
	require.Error(t, sc.Err())
	require.Contains(t, sc.Err().Error(), "connect")
}
