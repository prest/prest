package app

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mock"
	"github.com/prest/prest/v2/config"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

type migrateDBAdapter struct {
	*mock.Mock
	db *sqlx.DB
}

func (a *migrateDBAdapter) DB() (*sqlx.DB, error) {
	return a.db, nil
}

type countingEnsurer struct {
	*mock.Mock
	authCalls int
}

func (e *countingEnsurer) EnsureAuthTable(context.Context) error {
	e.authCalls++
	return nil
}

func (e *countingEnsurer) EnsureQueriesTable(context.Context) error {
	return nil
}

func TestEnsureSchemaMigrated_MixedRegistry(t *testing.T) {
	t.Parallel()

	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	sqlxDB := sqlx.NewDb(db, "postgres")
	t.Cleanup(func() { _ = sqlxDB.Close() })

	sqlMock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"\."prest_users"`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	pg := &migrateDBAdapter{Mock: mock.New(t), db: sqlxDB}
	shop := &countingEnsurer{Mock: mock.New(t)}

	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("pg", pg))
	require.NoError(t, registry.Register("shop", shop))

	cfg := &config.Prest{
		Engine: config.EngineMySQL,
		Databases: []config.DatabaseConf{
			{Alias: "pg", Engine: config.EnginePostgres},
			{Alias: "shop", Engine: config.EngineMySQL},
		},
		AuthEnabled:          true,
		AuthMigrateOnStartup: true,
		AuthSchema:           "public",
		AuthTable:            "prest_users",
	}

	require.NoError(t, ensureSchemaMigrated(cfg, registry))
	require.NoError(t, sqlMock.ExpectationsWereMet())
	require.Equal(t, 1, shop.authCalls)
}

func TestEnsureSchemaMigrated_RootMySQLEnsurerOnly(t *testing.T) {
	t.Parallel()

	shop := &countingEnsurer{Mock: mock.New(t)}
	registry := adapters.NewRegistry()
	require.NoError(t, registry.Register("shop", shop))

	cfg := &config.Prest{
		Engine:               config.EngineMySQL,
		AuthEnabled:          true,
		AuthMigrateOnStartup: true,
	}

	require.NoError(t, ensureSchemaMigrated(cfg, registry))
	require.Equal(t, 1, shop.authCalls)
}
