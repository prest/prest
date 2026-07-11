package app_test

import (
	"testing"

	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestEnsureAuthTable(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	defer sqlxDB.Close()

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"\."prest_users"`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	cfg := &config.Prest{
		AuthSchema: "public",
		AuthTable:  "prest_users",
	}
	require.NoError(t, app.EnsureAuthTable(cfg, sqlxDB))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureQueriesTable(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	sqlxDB := sqlx.NewDb(db, "postgres")
	defer sqlxDB.Close()

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"\."prest_queries"`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`CREATE INDEX IF NOT EXISTS "prest_queries_location_idx" ON "public"\."prest_queries"`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	cfg := &config.Prest{
		QueriesConf: config.QueriesConf{
			Schema: "public",
			Table:  "prest_queries",
		},
	}
	require.NoError(t, app.EnsureQueriesTable(cfg, sqlxDB))
	require.NoError(t, mock.ExpectationsWereMet())
}
