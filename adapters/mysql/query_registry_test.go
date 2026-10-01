package mysql

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func TestScanFilesystemQueriesIdentity(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "itest", "nested"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "bad name"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "itest", "get.read.sql"), []byte("select 1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "itest", "get.write.sql"), []byte("insert 1"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "itest", "nested", "get.read.sql"), []byte("select nested"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad name", "get.read.sql"), []byte("select bad"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "itest", "bad name.read.sql"), []byte("select unsafe"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "itest", "get.sql"), []byte("select plain"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "get.read.sql"), []byte("select root"), 0o644))

	got, err := scanFilesystemQueries(dir)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "itest", got[0].Location)
	require.Equal(t, "get", got[0].Name)
	require.Equal(t, "select 1", got[0].ReadSQL)
	require.Equal(t, "insert 1", got[0].WriteSQL)
	require.Empty(t, got[0].UpdateSQL)
	require.Empty(t, got[0].DeleteSQL)
}

func TestImportFromFilesystemLookupError(t *testing.T) {
	a, mock := withFlex(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "itest"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "itest", "get.read.sql"), []byte("select 1"), 0o644))

	driverErr := errors.New("connection reset")
	mock.ExpectQuery("name = ?").WillReturnError(driverErr)
	report, err := a.ImportFromFilesystem(ctx, dir, "")
	require.ErrorIs(t, err, driverErr)
	require.Equal(t, 0, report.Inserted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImportFromFilesystemInsertsOnNoRows(t *testing.T) {
	a, mock := withFlex(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "itest"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "itest", "get.read.sql"), []byte("select 1"), 0o644))

	mock.ExpectQuery("name = ?").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) AS new").WillReturnResult(sqlmock.NewResult(1, 1))
	report, err := a.ImportFromFilesystem(ctx, dir, "")
	require.NoError(t, err)
	require.Equal(t, 1, report.Inserted)
	require.NoError(t, mock.ExpectationsWereMet())
}
