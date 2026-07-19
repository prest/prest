package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mock"
	"github.com/prest/prest/v2/adapters/postgres"
	"github.com/prest/prest/v2/config"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

type dbAdapter struct {
	*mock.Mock
	db *sqlx.DB
}

func (a *dbAdapter) DB() (*sqlx.DB, error) {
	return a.db, nil
}

type failingDBAdapter struct {
	*mock.Mock
	err error
}

func (a *failingDBAdapter) DB() (*sqlx.DB, error) {
	return nil, a.err
}

type queryRegistryAdapter struct {
	*dbAdapter
	importFn func(ctx context.Context, queriesPath, policy string) (adapters.ImportReport, error)
}

func (a *queryRegistryAdapter) ListQueries(context.Context, string, string) ([]adapters.StoredQuery, error) {
	return nil, nil
}

func (a *queryRegistryAdapter) GetQuery(context.Context, string, string, string) (adapters.StoredQuery, error) {
	return adapters.StoredQuery{}, nil
}

func (a *queryRegistryAdapter) UpsertQuery(context.Context, adapters.StoredQuery) error {
	return nil
}

func (a *queryRegistryAdapter) DeleteQuery(context.Context, string, string, string) error {
	return nil
}

func (a *queryRegistryAdapter) ImportFromFilesystem(ctx context.Context, queriesPath, policy string) (adapters.ImportReport, error) {
	if a.importFn != nil {
		return a.importFn(ctx, queriesPath, policy)
	}
	return adapters.ImportReport{}, nil
}

var _ adapters.QueryRegistry = (*queryRegistryAdapter)(nil)

func newDBAdapter(t *testing.T) (*dbAdapter, sqlmock.Sqlmock) {
	t.Helper()

	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "postgres")
	t.Cleanup(func() { _ = sqlxDB.Close() })

	return &dbAdapter{Mock: mock.New(t), db: sqlxDB}, sqlMock
}

func expectAuthTableMigration(sqlMock sqlmock.Sqlmock) {
	sqlMock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"\."prest_users"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func expectQueriesTableMigration(sqlMock sqlmock.Sqlmock) {
	sqlMock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"\."prest_queries"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	sqlMock.ExpectExec(`CREATE INDEX IF NOT EXISTS "prest_queries_location_idx" ON "public"\."prest_queries"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

// stubDBConnect replaces sqlx.Connect for serial tests. Callers must not use t.Parallel().
func stubDBConnect(t *testing.T, fn func(driverName, dataSourceName string) (*sqlx.DB, error)) {
	t.Helper()
	restore := postgres.SetDBConnectForTest(fn)
	t.Cleanup(restore)
}

func newPingableSQLMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	// Do not enable MonitorPings: sqlmock treats Ping as a no-op unless monitored,
	// which matches how adapters/postgres connection tests stub dbConnect.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	t.Cleanup(func() { _ = sqlxDB.Close() })
	return sqlxDB, mock
}

func expectTimescaleExtension(mock sqlmock.Sqlmock, exists bool) {
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_extension WHERE extname='timescaledb'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

// stubPostgresConnect makes postgres.Connect succeed (single call; no Timescale attempt).
func stubPostgresConnect(t *testing.T) {
	t.Helper()
	stubDBConnect(t, func(_, _ string) (*sqlx.DB, error) {
		sqlxDB, _ := newPingableSQLMock(t)
		return sqlxDB, nil
	})
}

// stubPostgresFallbackConnect makes Timescale Connect fail, then postgres Connect succeed.
// Odd calls are Timescale attempts; even calls are postgres fallbacks.
func stubPostgresFallbackConnect(t *testing.T) {
	t.Helper()
	n := 0
	stubDBConnect(t, func(_, _ string) (*sqlx.DB, error) {
		n++
		if n%2 == 1 {
			return nil, errors.New("timescale unavailable")
		}
		sqlxDB, _ := newPingableSQLMock(t)
		return sqlxDB, nil
	})
}

// stubTimescaleConnect makes TimescaleDB detection succeed.
func stubTimescaleConnect(t *testing.T) {
	t.Helper()
	stubDBConnect(t, func(_, _ string) (*sqlx.DB, error) {
		sqlxDB, mock := newPingableSQLMock(t)
		expectTimescaleExtension(mock, true)
		return sqlxDB, nil
	})
}

func connectablePrest(database string) *config.Prest {
	return &config.Prest{
		PGHost:        "localhost",
		PGPort:        5432,
		PGUser:        "prest",
		PGDatabase:    database,
		PGSSLMode:     "disable",
		PGMaxIdleConn: 2,
		PGMaxOpenConn: 5,
	}
}

func baseDBConf(alias string) config.DatabaseConf {
	return config.DatabaseConf{
		Alias:       alias,
		Host:        "localhost",
		Port:        5432,
		User:        "prest",
		Pass:        "prest",
		Database:    alias + "_db",
		SSL:         config.DatabaseSSLConf{Mode: "disable"},
		MaxOpenConn: 5,
		MaxIdleConn: 2,
	}
}
