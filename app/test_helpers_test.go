package app_test

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mock"

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
