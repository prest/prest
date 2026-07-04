package postgres

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func withSQLMockPing(t *testing.T) (*postgres, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
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

func TestConnect_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMockPing(t)

	mock.ExpectPing()
	err := adapter.Connect()
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConnect_GetError(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")

	err := adapter.Connect()
	require.Error(t, err)
	require.Contains(t, err.Error(), "connect")
}

func TestPing_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMockPing(t)

	mock.ExpectPing()
	err := adapter.Ping(context.Background())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPing_Error(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMockPing(t)

	mock.ExpectPing().WillReturnError(errors.New("ping failed"))
	err := adapter.Ping(context.Background())
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDB_ReturnsInjectedConnection(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	db, err := adapter.DB()
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStmt_Prepare_CacheHit(t *testing.T) {
	t.Parallel()

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

	sql := `SELECT 1`
	mock.ExpectPrepare(sql)
	stmt1, err := adapter.Prepare(sqlxDB, sql)
	require.NoError(t, err)
	stmt2, err := adapter.Prepare(sqlxDB, sql)
	require.NoError(t, err)
	require.Same(t, stmt1, stmt2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStmt_PrepareContext_CacheHit(t *testing.T) {
	t.Parallel()

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

	ctx := context.Background()
	sql := `SELECT 1`
	mock.ExpectPrepare(sql)
	stmt1, err := adapter.PrepareContext(ctx, sqlxDB, sql)
	require.NoError(t, err)
	stmt2, err := adapter.PrepareContext(ctx, sqlxDB, sql)
	require.NoError(t, err)
	require.Same(t, stmt1, stmt2)
	require.NoError(t, mock.ExpectationsWereMet())
}
