package postgres

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

const qualifiedQueriesTable = `"public"."prest_queries"`

var storedQueryColumns = []string{
	"id", "database_alias", "location", "name",
	"read_sql", "write_sql", "update_sql", "delete_sql",
	"description", "created_by", "created_at", "updated_at",
}

func queryRegistryTestConf() *config.Prest {
	cfg := defaultTestConf()
	cfg.QueriesConf = config.QueriesConf{
		Schema: "public",
		Table:  "prest_queries",
	}
	return cfg
}

func withQueryRegistryMock(t *testing.T) (*postgres, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	cfg := queryRegistryTestConf()
	pg := New(cfg).(*postgres)
	pg.conn.SetDatabase(defaultMockDB)
	pg.conn.InjectDBForTest(pg.conn.GetURI(defaultMockDB), sqlxDB)
	t.Cleanup(func() { pg.conn.ResetPoolForTest() })
	pg.ClearStmt()
	t.Cleanup(pg.ClearStmt)
	return pg, mock
}

func sampleStoredQueryRow() *sqlmock.Rows {
	return sqlmock.NewRows(storedQueryColumns).
		AddRow(int64(1), "", "fulltable", "get_all",
			"SELECT 1", "INSERT 1", nil, nil,
			"desc", "alice", "2024-01-01", "2024-01-02")
}

func TestQueriesTable(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(queryRegistryTestConf())
	schema, table := adapter.queriesTable()
	require.Equal(t, "public", schema)
	require.Equal(t, "prest_queries", table)
}

func TestQueryLookupAliases(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{""}, queryLookupAliases(""))
	require.Equal(t, []string{"mydb", ""}, queryLookupAliases("mydb"))
}

func TestQualifiedQueriesTable(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(queryRegistryTestConf())
	qTable, err := adapter.qualifiedQueriesTable()
	require.NoError(t, err)
	require.Equal(t, qualifiedQueriesTable, qTable)

	badSchema := testAdapter(&config.Prest{
		QueriesConf: config.QueriesConf{Schema: "bad;schema", Table: "prest_queries"},
	})
	_, err = badSchema.qualifiedQueriesTable()
	require.Error(t, err)

	badTable := testAdapter(&config.Prest{
		QueriesConf: config.QueriesConf{Schema: "public", Table: "bad;table"},
	})
	_, err = badTable.qualifiedQueriesTable()
	require.Error(t, err)
}

func TestListQueries_InvalidTableConfig(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		QueriesConf: config.QueriesConf{Schema: "public", Table: "bad;table"},
	})
	_, err := adapter.ListQueries(context.Background(), "", "")
	require.Error(t, err)
}

func TestNullString(t *testing.T) {
	t.Parallel()

	require.Nil(t, nullString(""))
	require.Equal(t, "x", nullString("x"))
}

func TestHasAnyVerbSQL(t *testing.T) {
	t.Parallel()

	require.False(t, hasAnyVerbSQL(adapters.StoredQuery{}))
	require.True(t, hasAnyVerbSQL(adapters.StoredQuery{ReadSQL: "SELECT 1"}))
	require.True(t, hasAnyVerbSQL(adapters.StoredQuery{WriteSQL: "INSERT 1"}))
	require.True(t, hasAnyVerbSQL(adapters.StoredQuery{UpdateSQL: "UPDATE 1"}))
	require.True(t, hasAnyVerbSQL(adapters.StoredQuery{DeleteSQL: "DELETE 1"}))
}

func TestValidateQueryIdentity(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateQueryIdentity("", "loc", "name"))
	require.NoError(t, validateQueryIdentity("my-db", "loc", "name"))

	require.Error(t, validateQueryIdentity("", "bad loc", "name"))
	require.Error(t, validateQueryIdentity("", "loc", "bad name"))
	require.Error(t, validateQueryIdentity("bad;db", "loc", "name"))
}

func TestDiffStoredQuery_NoChange(t *testing.T) {
	t.Parallel()

	existing := adapters.StoredQuery{ReadSQL: "SELECT 1", WriteSQL: "INSERT 1"}
	incoming := adapters.StoredQuery{ReadSQL: "SELECT 1"}

	changed, conflict, err := diffStoredQuery(existing, incoming)
	require.NoError(t, err)
	require.False(t, changed)
	require.False(t, conflict)
	require.Empty(t, diffColumns(existing, incoming))
}

func TestDiffStoredQuery_NewColumnNoConflict(t *testing.T) {
	t.Parallel()

	existing := adapters.StoredQuery{}
	incoming := adapters.StoredQuery{ReadSQL: "SELECT 1"}

	changed, conflict, err := diffStoredQuery(existing, incoming)
	require.NoError(t, err)
	require.True(t, changed)
	require.False(t, conflict)
	require.Equal(t, []string{"read_sql"}, diffColumns(existing, incoming))
}

func TestMergeStoredQuery_AllColumns(t *testing.T) {
	t.Parallel()

	existing := adapters.StoredQuery{
		ReadSQL: "r0", WriteSQL: "w0", UpdateSQL: "u0", DeleteSQL: "d0",
	}
	incoming := adapters.StoredQuery{
		ReadSQL: "r1", WriteSQL: "w1", UpdateSQL: "u1", DeleteSQL: "d1",
	}
	merged := mergeStoredQuery(existing, incoming)
	require.Equal(t, "r1", merged.ReadSQL)
	require.Equal(t, "w1", merged.WriteSQL)
	require.Equal(t, "u1", merged.UpdateSQL)
	require.Equal(t, "d1", merged.DeleteSQL)
}

func TestListQueries_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM ` + qualifiedQueriesTable).
		WillReturnRows(sampleStoredQueryRow())

	queries, err := adapter.ListQueries(ctx, "", "")
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.Equal(t, "get_all", queries[0].Name)
	require.Equal(t, "SELECT 1", queries[0].ReadSQL)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListQueries_WithFilters(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("mydb", "fulltable").
		WillReturnRows(sampleStoredQueryRow())

	queries, err := adapter.ListQueries(ctx, "mydb", "fulltable")
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListQueries_QueryError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM ` + qualifiedQueriesTable).
		WillReturnError(errors.New("query failed"))

	_, err := adapter.ListQueries(ctx, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "list queries")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListQueries_ScanError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM ` + qualifiedQueriesTable).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	_, err := adapter.ListQueries(ctx, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "scan query")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListQueries_RowsErr(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM ` + qualifiedQueriesTable).
		WillReturnRows(sampleStoredQueryRow().CloseError(errors.New("rows err")))

	_, err := adapter.ListQueries(ctx, "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "list queries rows")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListQueries_DBConnectError(t *testing.T) {
	restore := connection.SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, errors.New("connect failed")
	})
	t.Cleanup(restore)

	adapter := New(queryRegistryTestConf()).(*postgres)
	_, err := adapter.ListQueries(context.Background(), "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "connect")
}

func TestGetQuery_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnRows(sampleStoredQueryRow())

	q, err := adapter.GetQuery(ctx, "", "fulltable", "get_all")
	require.NoError(t, err)
	require.Equal(t, "get_all", q.Name)
	require.Equal(t, "SELECT 1", q.ReadSQL)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetQuery_AliasFallback(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("mydb", "fulltable", "get_all").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnRows(sampleStoredQueryRow())

	q, err := adapter.GetQuery(ctx, "mydb", "fulltable", "get_all")
	require.NoError(t, err)
	require.Equal(t, "get_all", q.Name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetQuery_NotFound(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "missing").
		WillReturnError(sql.ErrNoRows)

	_, err := adapter.GetQuery(ctx, "", "fulltable", "missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "query not found")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetQuery_NotFoundWithAliasFallback(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("mydb", "fulltable", "missing").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "missing").
		WillReturnError(sql.ErrNoRows)

	_, err := adapter.GetQuery(ctx, "mydb", "fulltable", "missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "query not found")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetQuery_DBConnectError(t *testing.T) {
	restore := connection.SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, errors.New("connect failed")
	})
	t.Cleanup(restore)

	adapter := New(queryRegistryTestConf()).(*postgres)
	_, err := adapter.GetQuery(context.Background(), "", "fulltable", "get_all")
	require.Error(t, err)
}

func TestGetQuery_InvalidTableConfig(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		QueriesConf: config.QueriesConf{Schema: "public", Table: "bad;table"},
	})
	_, err := adapter.GetQuery(context.Background(), "", "fulltable", "get_all")
	require.Error(t, err)
}

func TestGetQuery_QueryError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnError(errors.New("db error"))

	_, err := adapter.GetQuery(ctx, "", "fulltable", "get_all")
	require.Error(t, err)
	require.Contains(t, err.Error(), "get query")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertQuery_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`INSERT INTO `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all", "SELECT 1", nil, nil, nil, nil, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := adapter.UpsertQuery(ctx, adapters.StoredQuery{
		Location: "fulltable",
		Name:     "get_all",
		ReadSQL:  "SELECT 1",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertQuery_WithAllFields(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`INSERT INTO `+qualifiedQueriesTable).
		WithArgs("mydb", "fulltable", "get_all",
			"SELECT 1", "INSERT 1", "UPDATE 1", "DELETE 1", "desc", "alice").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := adapter.UpsertQuery(ctx, adapters.StoredQuery{
		DatabaseAlias: "mydb",
		Location:      "fulltable",
		Name:          "get_all",
		ReadSQL:       "SELECT 1",
		WriteSQL:      "INSERT 1",
		UpdateSQL:     "UPDATE 1",
		DeleteSQL:     "DELETE 1",
		Description:   "desc",
		CreatedBy:     "alice",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertQuery_ValidationErrors(t *testing.T) {
	t.Parallel()

	adapter, _ := withQueryRegistryMock(t)
	ctx := context.Background()

	err := adapter.UpsertQuery(ctx, adapters.StoredQuery{
		Location: "bad loc",
		Name:     "name",
		ReadSQL:  "SELECT 1",
	})
	require.Error(t, err)

	err = adapter.UpsertQuery(ctx, adapters.StoredQuery{
		Location: "loc",
		Name:     "name",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "verb SQL")
}

func TestUpsertQuery_DBConnectError(t *testing.T) {
	restore := connection.SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, errors.New("connect failed")
	})
	t.Cleanup(restore)

	adapter := New(queryRegistryTestConf()).(*postgres)
	err := adapter.UpsertQuery(context.Background(), adapters.StoredQuery{
		Location: "fulltable",
		Name:     "get_all",
		ReadSQL:  "SELECT 1",
	})
	require.Error(t, err)
}

func TestUpsertQuery_InvalidTableConfig(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		QueriesConf: config.QueriesConf{Schema: "public", Table: "bad;table"},
	})
	err := adapter.UpsertQuery(context.Background(), adapters.StoredQuery{
		Location: "fulltable",
		Name:     "get_all",
		ReadSQL:  "SELECT 1",
	})
	require.Error(t, err)
}

func TestUpsertQuery_ExecError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`INSERT INTO `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all", "SELECT 1", nil, nil, nil, nil, nil).
		WillReturnError(errors.New("exec failed"))

	err := adapter.UpsertQuery(ctx, adapters.StoredQuery{
		Location: "fulltable",
		Name:     "get_all",
		ReadSQL:  "SELECT 1",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "upsert query")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteQuery_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`DELETE FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := adapter.DeleteQuery(ctx, "", "fulltable", "get_all")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteQuery_NotFound(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`DELETE FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := adapter.DeleteQuery(ctx, "", "fulltable", "missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "query not found")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteQuery_ExecError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`DELETE FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnError(errors.New("exec failed"))

	err := adapter.DeleteQuery(ctx, "", "fulltable", "get_all")
	require.Error(t, err)
	require.Contains(t, err.Error(), "delete query")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteQuery_RowsAffectedError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`DELETE FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected failed")))

	err := adapter.DeleteQuery(ctx, "", "fulltable", "get_all")
	require.Error(t, err)
	require.Contains(t, err.Error(), "rows affected")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteQuery_ValidationError(t *testing.T) {
	t.Parallel()

	adapter, _ := withQueryRegistryMock(t)
	err := adapter.DeleteQuery(context.Background(), "", "bad loc", "name")
	require.Error(t, err)
}

func TestDeleteQuery_DBConnectError(t *testing.T) {
	restore := connection.SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, errors.New("connect failed")
	})
	t.Cleanup(restore)

	adapter := New(queryRegistryTestConf()).(*postgres)
	err := adapter.DeleteQuery(context.Background(), "", "fulltable", "get_all")
	require.Error(t, err)
}

func TestDeleteQuery_InvalidTableConfig(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		QueriesConf: config.QueriesConf{Schema: "public", Table: "bad;table"},
	})
	err := adapter.DeleteQuery(context.Background(), "", "fulltable", "get_all")
	require.Error(t, err)
}

func TestPatchQuery_NoColumns(t *testing.T) {
	t.Parallel()

	adapter, _ := withQueryRegistryMock(t)
	err := adapter.patchQuery(context.Background(), adapters.StoredQuery{}, nil)
	require.NoError(t, err)
}

func TestPatchQuery_Success(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE `+qualifiedQueriesTable).
		WithArgs("SELECT 2", "", "fulltable", "get_all").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := adapter.patchQuery(ctx, adapters.StoredQuery{
		Location: "fulltable",
		Name:     "get_all",
		ReadSQL:  "SELECT 2",
	}, []string{"read_sql"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPatchQuery_ExecError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE `+qualifiedQueriesTable).
		WithArgs("SELECT 2", "", "fulltable", "get_all").
		WillReturnError(errors.New("patch failed"))

	err := adapter.patchQuery(ctx, adapters.StoredQuery{
		Location: "fulltable",
		Name:     "get_all",
		ReadSQL:  "SELECT 2",
	}, []string{"read_sql"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "patch query")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPatchQuery_AllVerbColumns(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE `+qualifiedQueriesTable).
		WithArgs("W", "U", "D", "", "fulltable", "get_all").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := adapter.patchQuery(ctx, adapters.StoredQuery{
		Location:  "fulltable",
		Name:      "get_all",
		WriteSQL:  "W",
		UpdateSQL: "U",
		DeleteSQL: "D",
	}, []string{"write_sql", "update_sql", "delete_sql"})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPatchQuery_DBConnectError(t *testing.T) {
	restore := connection.SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, errors.New("connect failed")
	})
	t.Cleanup(restore)

	adapter := New(queryRegistryTestConf()).(*postgres)
	err := adapter.patchQuery(context.Background(), adapters.StoredQuery{
		Location: "fulltable",
		Name:     "get_all",
		ReadSQL:  "SELECT 2",
	}, []string{"read_sql"})
	require.Error(t, err)
}

func TestPatchQuery_InvalidTableConfig(t *testing.T) {
	t.Parallel()

	adapter := testAdapter(&config.Prest{
		QueriesConf: config.QueriesConf{Schema: "public", Table: "bad;table"},
	})
	err := adapter.patchQuery(context.Background(), adapters.StoredQuery{
		Location: "fulltable",
		Name:     "get_all",
		ReadSQL:  "SELECT 2",
	}, []string{"read_sql"})
	require.Error(t, err)
}

func writeImportFixture(t *testing.T, readSQL string) string {
	t.Helper()
	dir := t.TempDir()
	locDir := filepath.Join(dir, "fulltable")
	require.NoError(t, os.MkdirAll(locDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(locDir, "get_all.read.sql"),
		[]byte(readSQL),
		0o600,
	))
	return dir
}

func TestImportFromFilesystem_Insert(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()
	dir := writeImportFixture(t, "SELECT 1")

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all", "SELECT 1", nil, nil, nil, nil, "filesystem-import").
		WillReturnResult(sqlmock.NewResult(1, 1))

	report, err := adapter.ImportFromFilesystem(ctx, dir, config.QueriesImportPolicyUpdate)
	require.NoError(t, err)
	require.Equal(t, 1, report.Inserted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFromFilesystem_SkipUnchanged(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()
	dir := writeImportFixture(t, "SELECT 1")

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnRows(sampleStoredQueryRow())

	report, err := adapter.ImportFromFilesystem(ctx, dir, config.QueriesImportPolicyUpdate)
	require.NoError(t, err)
	require.Equal(t, 1, report.Skipped)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFromFilesystem_Update(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()
	dir := writeImportFixture(t, "SELECT 2")

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnRows(sampleStoredQueryRow())
	mock.ExpectExec(`UPDATE `+qualifiedQueriesTable).
		WithArgs("SELECT 2", "", "fulltable", "get_all").
		WillReturnResult(sqlmock.NewResult(0, 1))

	report, err := adapter.ImportFromFilesystem(ctx, dir, config.QueriesImportPolicyUpdate)
	require.NoError(t, err)
	require.Equal(t, 1, report.Updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFromFilesystem_ConflictError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()
	dir := writeImportFixture(t, "SELECT 2")

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnRows(sampleStoredQueryRow())

	report, err := adapter.ImportFromFilesystem(ctx, dir, config.QueriesImportPolicyError)
	require.Error(t, err)
	require.Contains(t, err.Error(), "import conflict")
	require.Equal(t, 0, report.Updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFromFilesystem_ConflictSkip(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()
	dir := writeImportFixture(t, "SELECT 2")

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnRows(sampleStoredQueryRow())

	report, err := adapter.ImportFromFilesystem(ctx, dir, config.QueriesImportPolicySkip)
	require.NoError(t, err)
	require.Equal(t, 1, report.Skipped)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFromFilesystem_NotADirectory(t *testing.T) {
	t.Parallel()

	adapter, _ := withQueryRegistryMock(t)
	file := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	_, err := adapter.ImportFromFilesystem(context.Background(), file, config.QueriesImportPolicyUpdate)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a directory")
}

func TestImportFromFilesystem_PatchError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()
	dir := writeImportFixture(t, "SELECT 2")

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnRows(sampleStoredQueryRow())
	mock.ExpectExec(`UPDATE `+qualifiedQueriesTable).
		WithArgs("SELECT 2", "", "fulltable", "get_all").
		WillReturnError(errors.New("patch failed"))

	_, err := adapter.ImportFromFilesystem(ctx, dir, config.QueriesImportPolicyUpdate)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFromFilesystem_UpsertError(t *testing.T) {
	t.Parallel()

	adapter, mock := withQueryRegistryMock(t)
	ctx := context.Background()
	dir := writeImportFixture(t, "SELECT 1")

	mock.ExpectQuery(`FROM `+qualifiedQueriesTable).
		WithArgs("", "fulltable", "get_all").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`INSERT INTO ` + qualifiedQueriesTable).
		WillReturnError(errors.New("upsert failed"))

	_, err := adapter.ImportFromFilesystem(ctx, dir, config.QueriesImportPolicyUpdate)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListQueries_CtxDB(t *testing.T) {
	t.Parallel()

	cfg := queryRegistryTestConf()
	defaultDB, defaultMock, err := sqlmock.New()
	require.NoError(t, err)
	ctxDB, ctxMock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = defaultDB.Close()
		_ = ctxDB.Close()
	})

	pg := New(cfg).(*postgres)
	pg.conn.SetDatabase(defaultMockDB)
	pg.conn.InjectDBForTest(pg.conn.GetURI(defaultMockDB), sqlx.NewDb(defaultDB, "sqlmock"))
	pg.conn.InjectDBForTest(pg.conn.GetURI(contextMockDB), sqlx.NewDb(ctxDB, "sqlmock"))
	t.Cleanup(func() { pg.conn.ResetPoolForTest() })

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectQuery(`FROM ` + qualifiedQueriesTable).
		WillReturnRows(sampleStoredQueryRow())

	queries, err := pg.ListQueries(ctx, "", "")
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

func TestDiffStoredQuery(t *testing.T) {
	t.Parallel()

	existing := adapters.StoredQuery{ReadSQL: "SELECT 1", WriteSQL: "INSERT 1"}
	incoming := adapters.StoredQuery{ReadSQL: "SELECT 1", WriteSQL: "INSERT 2"}

	changed, conflict, err := diffStoredQuery(existing, incoming)
	require.NoError(t, err)
	require.True(t, changed)
	require.True(t, conflict)

	cols := diffColumns(existing, incoming)
	require.Equal(t, []string{"write_sql"}, cols)
}

func TestMergeStoredQuery_PreservesMissingFileColumns(t *testing.T) {
	t.Parallel()

	existing := adapters.StoredQuery{ReadSQL: "SELECT 1", DeleteSQL: "DELETE 1"}
	incoming := adapters.StoredQuery{WriteSQL: "INSERT 1"}

	merged := mergeStoredQuery(existing, incoming)
	require.Equal(t, "SELECT 1", merged.ReadSQL)
	require.Equal(t, "INSERT 1", merged.WriteSQL)
	require.Equal(t, "DELETE 1", merged.DeleteSQL)
}

func TestScanFilesystemQueries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	loc := "fulltable"
	locDir := filepath.Join(dir, loc)
	require.NoError(t, os.MkdirAll(locDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "get_all.read.sql"), []byte("SELECT 1"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "get_all.write.sql"), []byte("INSERT 1"), 0o600))

	queries, err := scanFilesystemQueries(dir)
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.Equal(t, "fulltable", queries[0].Location)
	require.Equal(t, "get_all", queries[0].Name)
	require.Equal(t, "SELECT 1", queries[0].ReadSQL)
	require.Equal(t, "INSERT 1", queries[0].WriteSQL)
	require.Equal(t, "filesystem-import", queries[0].CreatedBy)
}

func TestScanFilesystemQueries_MissingPath(t *testing.T) {
	t.Parallel()

	queries, err := scanFilesystemQueries(filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	require.Nil(t, queries)
}

func TestScanFilesystemQueries_NotDirectory(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))

	_, err := scanFilesystemQueries(file)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a directory")
}

func TestScanFilesystemQueries_IgnoresInvalidFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "orphan.sql"), []byte("x"), 0o600))

	locDir := filepath.Join(dir, "loc")
	require.NoError(t, os.MkdirAll(locDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "q.read.sql"), []byte("SELECT 1"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "notes.txt"), []byte("nope"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, ".read.sql"), []byte("empty"), 0o600))

	queries, err := scanFilesystemQueries(dir)
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.Equal(t, "q", queries[0].Name)
}

func TestScanFilesystemQueries_AllVerbSuffixes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	locDir := filepath.Join(dir, "api")
	require.NoError(t, os.MkdirAll(locDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "item.read.sql"), []byte("R"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "item.write.sql"), []byte("W"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "item.update.sql"), []byte("U"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(locDir, "item.delete.sql"), []byte("D"), 0o600))

	queries, err := scanFilesystemQueries(dir)
	require.NoError(t, err)
	require.Len(t, queries, 1)
	require.Equal(t, "R", queries[0].ReadSQL)
	require.Equal(t, "W", queries[0].WriteSQL)
	require.Equal(t, "U", queries[0].UpdateSQL)
	require.Equal(t, "D", queries[0].DeleteSQL)
}
