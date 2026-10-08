package mysql

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mysql/statements"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func TestWrapDriverAccessErrorsHideServerText(t *testing.T) {
	t.Parallel()
	for _, num := range []uint16{1142, 1044, 1049} {
		err := wrapDriver(&mysql.MySQLError{Number: num, Message: "denied for user 'prest'@'172.17.0.1'"})
		require.ErrorIs(t, err, adapters.ErrRelationNotFound, num)
		require.NotContains(t, err.Error(), "@", num)
		require.NotContains(t, err.Error(), "prest", num)
	}
	err := wrapDriver(&mysql.MySQLError{Number: 1146, Message: "Table 'shop.nope' doesn't exist"})
	require.ErrorIs(t, err, adapters.ErrRelationNotFound)
	require.Contains(t, err.Error(), "shop.nope")
}

func TestWrapDriverUnavailable(t *testing.T) {
	t.Parallel()
	dial := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	for _, in := range []error{dial, fmt.Errorf("connect: %w", dial), mysql.ErrInvalidConn} {
		err := wrapDriver(in)
		require.ErrorIs(t, err, adapters.ErrUnavailable)
		require.NotContains(t, err.Error(), "dial")
		require.NotContains(t, err.Error(), "refused")
	}
	// Already-wrapped errors are not wrapped twice.
	once := wrapDriver(dial)
	require.Equal(t, once.Error(), wrapDriver(once).Error())
	require.Equal(t, errBodyEmpty, wrapDriver(errBodyEmpty))
}

func TestQueryUnavailableSurfacesSentinel(t *testing.T) {
	a, mock := withMock(t)
	mock.ExpectQuery("SELECT 1").WillReturnError(mysql.ErrInvalidConn)
	sc := a.QueryCtx(context.Background(), "SELECT 1")
	require.ErrorIs(t, sc.Err(), adapters.ErrUnavailable)
}

func TestDBFromCtxConnectFailureIsUnavailable(t *testing.T) {
	restore := SetDBConnectForTest(func(string, string) (*sqlx.DB, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	})
	t.Cleanup(restore)
	a := New(testCfg()).(*Adapter)
	a.SetDatabase("shop")
	_, err := a.dbFromCtx(context.Background())
	require.ErrorIs(t, err, adapters.ErrUnavailable)
	require.NotContains(t, err.Error(), "dial")
	require.ErrorIs(t, a.Ping(context.Background()), adapters.ErrUnavailable)
}

func TestBatchInsertCopyDefaultMarkers(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `shop`.`items`(`name`,`qty`) VALUES(?,DEFAULT),(?,?)").
		WithArgs("a", "b", 2).
		WillReturnResult(sqlmock.NewResult(10, 2))
	mock.ExpectQuery("SELECT @@auto_increment_increment").
		WillReturnRows(sqlmock.NewRows([]string{"@@auto_increment_increment"}).AddRow(int64(1)))
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", "auto_increment"))
	mock.ExpectQuery("SELECT * FROM `shop`.`items` WHERE `id` IN (?,?)").
		WithArgs(int64(10), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "qty"}).AddRow(int64(10), "a", int64(1)).AddRow(int64(11), "b", int64(2)))
	mock.ExpectCommit()
	sc := a.BatchInsertCopyCtx(ctx, "alias", "shop", "items", []string{"`name`", "`qty`"}, "a", adapters.DefaultValue{}, "b", 2)
	require.NoError(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsertValuesDefaultMarkers(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `shop`.`items`(`name`,`qty`) VALUES(?,DEFAULT),(?,?)").
		WithArgs("a", "b", 2).
		WillReturnResult(sqlmock.NewResult(10, 2))
	mock.ExpectQuery("SELECT @@auto_increment_increment").
		WillReturnRows(sqlmock.NewRows([]string{"@@auto_increment_increment"}).AddRow(int64(1)))
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", "auto_increment"))
	mock.ExpectQuery("SELECT * FROM `shop`.`items` WHERE `id` IN (?,?)").
		WithArgs(int64(10), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "qty"}).AddRow(int64(10), "a", int64(1)).AddRow(int64(11), "b", int64(2)))
	mock.ExpectCommit()
	sc := a.BatchInsertValuesCtx(ctx, "INSERT INTO `shop`.`items`(`name`,`qty`) VALUES(?,DEFAULT),(?,?)", "a", adapters.DefaultValue{}, "b", 2)
	require.NoError(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
	require.False(t, errors.Is(sc.Err(), adapters.ErrUnavailable))
}

func TestBatchFallbackOmitsDefaultMarkers(t *testing.T) {
	// A record that left a column out must not echo the DEFAULT marker back as {}.
	buf, err := batchFallback([]string{"a", "b"}, []any{1, adapters.DefaultValue{}, adapters.DefaultValue{}, 2}, nil, 0)
	require.NoError(t, err)
	require.JSONEq(t, `[{"a":1},{"b":2}]`, string(buf))
}
