package mysql

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mysql/statements"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func testCfg() *config.Prest {
	return &config.Prest{
		Engine:     config.EngineMySQL,
		PGHost:     "db.internal",
		PGPort:     3306,
		PGUser:     "app",
		PGPass:     "pw",
		PGDatabase: "shop",
		PGSSLMode:  "disable",
		AuthSchema: "shop",
		AuthTable:  "prest_users",
		QueriesConf: config.QueriesConf{
			Schema: "shop",
			Table:  "prest_queries",
		},
	}
}

func withMock(t *testing.T) (*Adapter, sqlmock.Sqlmock) {
	t.Helper()
	raw, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	a := New(testCfg()).(*Adapter)
	a.SetDatabase("shop")
	require.NoError(t, a.conn.InjectDBForTest(sqlx.NewDb(raw, "sqlmock")))
	return a, mock
}

func req(raw string) *http.Request {
	r, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		panic(err)
	}
	return r
}

func TestWhereOrderJoinSelect(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)

	where, vals, err := a.WhereByRequest(req("/t?name=$eq.ada"), 9)
	require.NoError(t, err)
	require.Equal(t, "`name` = ?", where)
	require.Equal(t, []any{"ada"}, vals)
	where, vals, err = a.WhereByRequest(req("/t?id=$in.1,2"), 1)
	require.NoError(t, err)
	require.Equal(t, "`id` IN (?,?)", where)
	require.Equal(t, []any{"1", "2"}, vals)
	where, _, err = a.WhereByRequest(req("/t?nick=$ilike.a"), 1)
	require.NoError(t, err)
	require.Equal(t, "`nick` LIKE ?", where)
	where, vals, err = a.WhereByRequest(req("/t?tags=$any.x,y"), 1)
	require.NoError(t, err)
	require.Equal(t, "`tags` IN (?,?)", where)
	require.Equal(t, []any{"x", "y"}, vals)

	_, _, err = a.WhereByRequest(req("/t?body=$tsquery.hello"), 1)
	require.ErrorIs(t, err, errUnsupported)
	_, _, err = a.WhereByRequest(req("/t?path=$ltreelanc.a"), 1)
	require.ErrorIs(t, err, errUnsupported)
	_, _, err = a.WhereByRequest(req("/t?n=$all.1,2"), 1)
	require.ErrorIs(t, err, errUnsupported)

	where, vals, err = a.WhereByRequest(req("/t?meta->>kind:jsonb=$eq.hat"), 1)
	require.NoError(t, err)
	require.Equal(t, "`meta`->>'$.kind' = ?", where)
	require.Equal(t, []any{"hat"}, vals)

	order, err := a.OrderByRequest(req("/t?_order=name,-id"))
	require.NoError(t, err)
	require.Equal(t, " ORDER BY `name` , `id` DESC", order)
	_, err = a.OrderByRequest(req("/t?_korder=embedding:l2:1"))
	require.ErrorIs(t, err, errUnsupported)

	joins, err := a.JoinByRequest(req("/t?_join=INNER:orders:orders.id:$eq:items.id"))
	require.NoError(t, err)
	require.Equal(t, []string{" INNER JOIN `orders` ON `orders`.`id` = `items`.`id` "}, joins)
	_, err = a.JoinByRequest(req("/t?_join=FULL:orders:orders.id:$eq:items.id"))
	require.ErrorIs(t, err, errUnsupported)

	sel, err := a.SelectFields([]string{"id", "sum:price:total"})
	require.NoError(t, err)
	require.Equal(t, "SELECT `id`,SUM(`price`) AS `total` FROM", sel)
	require.Equal(t, "SELECT `id` FROM `shop`.`items`", a.SelectSQL("SELECT `id` FROM", "alias", "shop", "items"))
	require.Contains(t, a.InsertSQL("alias", "shop", "items", "`name`", "(?)"), "INSERT INTO `shop`.`items`(`name`) VALUES(?)")

	page, err := a.PaginateIfPossible(req("/t?_page=2&_page_size=5"))
	require.NoError(t, err)
	require.Equal(t, "LIMIT 5 OFFSET 5", page)
	require.Equal(t, "GROUP BY `name`", a.GroupByClause(req("/t?_groupby=name")))
	tb, err := a.TimeBucketClause(req("/t"))
	require.NoError(t, err)
	require.Empty(t, tb)

	count, err := a.CountByRequest(req("/t?_count=id"))
	require.NoError(t, err)
	require.Equal(t, "SELECT COUNT(`id`) FROM", count)
}

func TestParseInsertSortsAndMarshalsSlice(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	r := req("/t")
	r.Body = io.NopCloser(strings.NewReader(`{"b":[1,2],"a":"x"}`))
	names, ph, vals, err := a.ParseInsertRequest(r)
	require.NoError(t, err)
	require.Equal(t, "`a`, `b`", names)
	require.Equal(t, "(?,?)", ph)
	require.Equal(t, "x", vals[0])
	require.JSONEq(t, "[1,2]", string(vals[1].([]byte)))
}

func TestCatalogSQL(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	q, count := a.DatabaseClause(req("/databases"))
	require.False(t, count)
	require.Contains(t, q, "SCHEMA_NAME AS datname")
	require.Contains(t, q, "information_schema")
	q, count = a.DatabaseClause(req("/databases?_count=datname"))
	require.True(t, count)
	require.Contains(t, q, "COUNT(datname)")
	require.Contains(t, a.DatabaseWhere("`datname` = ?"), "WHERE 1=1 AND")
	require.Equal(t, " ORDER BY datname ASC", a.DatabaseOrderBy("", false))
	require.Empty(t, a.DatabaseOrderBy("", true))

	s, _ := a.SchemaClause(req("/schemas"))
	require.Contains(t, s, "schema_name")
	require.Contains(t, a.TableClause(), "`schema`")
	require.Contains(t, a.TableClause(), "BASE TABLE")
	require.Contains(t, a.TableWhere("`name` = ?"), "WHERE 1=1 AND")
	require.Contains(t, a.SchemaTablesClause(), "? AS `database`")
	require.Contains(t, a.SchemaTablesWhere("`name` = ?"), "TABLE_SCHEMA = ?")
	require.Contains(t, statements.ShowColumns, "is_generated")
	require.Contains(t, statements.ShowTableWhere, "TABLE_NAME = ?")
}

func TestQueryJSONScan(t *testing.T) {
	a, mock := withMock(t)
	when := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	mock.ExpectQuery("SELECT `id` FROM `shop`.`items`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "doc", "seen"}).
			AddRow(int64(1), []byte("12.50"), []byte(`{"a":1}`), when).
			AddRow(nil, []byte("true"), []byte("hello"), nil))
	sc := a.Query("SELECT `id` FROM `shop`.`items`")
	require.NoError(t, sc.Err())
	body := string(sc.Bytes())
	require.Contains(t, body, `"id":1`)
	require.Contains(t, body, `"amount":"12.50"`)
	require.Contains(t, body, `"doc":{"a":1}`)
	require.Contains(t, body, `"seen":"2024-01-02T03:04:05Z"`)
	require.Contains(t, body, `"hello"`)
	require.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	sc = a.Query("SELECT 1")
	require.NoError(t, sc.Err())
	require.Equal(t, "[]", string(sc.Bytes()))

	mock.ExpectQuery("SELECT COUNT(*) FROM `shop`.`items`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))
	sc = a.QueryCount("SELECT COUNT(*) FROM `shop`.`items`")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"count":3}`, string(sc.Bytes()))
}

func TestQueryRelationNotFound(t *testing.T) {
	a, mock := withMock(t)
	mock.ExpectQuery("SELECT 1").WillReturnError(&mysql.MySQLError{Number: 1146, Message: "Table 'shop.missing' doesn't exist"})
	sc := a.Query("SELECT 1")
	require.ErrorIs(t, sc.Err(), adapters.ErrRelationNotFound)
}

func TestInsertUpdateDelete(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `shop`.`items`(`name`) VALUES(?)").
		WithArgs("ada").
		WillReturnResult(sqlmock.NewResult(4, 1))
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", "auto_increment"))
	mock.ExpectQuery("SELECT * FROM `shop`.`items` WHERE `id`= ?").
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(4), "ada"))
	mock.ExpectCommit()
	sc := a.InsertCtx(ctx, "INSERT INTO `shop`.`items`(`name`) VALUES(?)", "ada")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"id":4,"name":"ada"}`, string(sc.Bytes()))

	mock.ExpectExec("UPDATE `shop`.`items` SET `name`=?").
		WithArgs("bea").
		WillReturnResult(sqlmock.NewResult(0, 1))
	sc = a.UpdateCtx(ctx, "UPDATE `shop`.`items` SET `name`=?", "bea")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `{"rows_affected":1}`, string(sc.Bytes()))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `shop`.`items` WHERE `id`=? FOR UPDATE").
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(4)))
	mock.ExpectExec("UPDATE `shop`.`items` SET `name`=? WHERE `id`=?").
		WithArgs("bea", int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT `id`, `name` FROM `shop`.`items` WHERE `id` = ?").
		WithArgs([]byte("4")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(4), "bea"))
	mock.ExpectCommit()
	sc = a.UpdateCtx(ctx, "UPDATE `shop`.`items` SET `name`=? WHERE `id`=? RETURNING `id`, `name`", "bea", int64(4))
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"id":4,"name":"bea"}]`, string(sc.Bytes()))

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT `id` FROM `shop`.`items` WHERE `id`=?").
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(4)))
	mock.ExpectExec("DELETE FROM `shop`.`items` WHERE `id`=?").
		WithArgs(int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	sc = a.DeleteCtx(ctx, "DELETE FROM `shop`.`items` WHERE `id`=? RETURNING `id`", int64(4))
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"id":4}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsertCopy(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `shop`.`items`(`name`) VALUES(?),(?)").
		WithArgs("a", "b").
		WillReturnResult(sqlmock.NewResult(10, 2))
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", "auto_increment"))
	mock.ExpectQuery("SELECT * FROM `shop`.`items` WHERE `id` IN (?,?)").
		WithArgs(int64(10), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(int64(10), "a").AddRow(int64(11), "b"))
	mock.ExpectCommit()
	sc := a.BatchInsertCopyCtx(ctx, "alias", "shop", "items", []string{"`name`"}, "a", "b")
	require.NoError(t, sc.Err())
	require.Contains(t, string(sc.Bytes()), `"name":"a"`)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScriptTemplatePlaceholders(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	sql, vals, err := a.ParseScriptTemplate("q", `SELECT * FROM {{ident "tbl"}} WHERE id = {{sqlVal "id"}}`, map[string]any{
		"tbl": "items",
		"id":  7,
	})
	require.NoError(t, err)
	require.Equal(t, "SELECT * FROM `items` WHERE id = ?", sql)
	require.Equal(t, []any{7}, vals)
}

func TestPingAndUnknownAlias(t *testing.T) {
	raw, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	a := New(testCfg()).(*Adapter)
	a.SetDatabase("shop")
	require.NoError(t, a.conn.InjectDBForTest(sqlx.NewDb(raw, "sqlmock")))
	mock.ExpectPing()
	require.NoError(t, a.Ping(context.Background()))
	_, err = a.dbFromCtx(context.WithValue(context.Background(), pctx.DBNameKey, "other"))
	require.Error(t, err)
	require.Equal(t, []string{"shop"}, a.Aliases())
	require.Equal(t, "shop", a.PhysicalName("shop"))
}

func TestPermissions(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.AccessConf.Restrict = true
	cfg.AccessConf.Tables = []config.TablesConf{{
		Name: "items", Permissions: []string{"read"}, Fields: []string{"id"},
	}}
	a := New(cfg).(*Adapter)
	require.True(t, a.TablePermissions("shop", "shop", "items", "read", ""))
	require.False(t, a.TablePermissions("shop", "shop", "items", "write", ""))
	fields, err := a.FieldsPermissions(req("/t?_select=id,name"), "shop", "shop", "items", "read", "")
	require.NoError(t, err)
	require.Equal(t, []string{"id"}, fields)
	require.True(t, a.ScriptPermissions(context.Background(), "shop", "r", "q", "read", ""))
}

func TestFieldsPermissionsUnwrapsColumnError(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	_, err := a.FieldsPermissions(req("/t?_groupby=name&_select=nope:id"), "shop", "shop", "items", "read", "")
	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidGroupFn)
	require.NotNil(t, errors.Unwrap(err))
	require.Contains(t, err.Error(), "error on parse columns from request")
}

func TestUpdateReturningCapturedKey(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")

	mock.ExpectBegin()
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", "auto_increment"))
	mock.ExpectQuery("SELECT `id` FROM `shop`.`items` WHERE `status`=? FOR UPDATE").
		WithArgs("open").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectExec("UPDATE `shop`.`items` SET `status`=? WHERE `status`=?").
		WithArgs("closed", "open").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT `id` FROM `shop`.`items` WHERE `id` = ?").
		WithArgs([]byte("7")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectCommit()
	sc := a.UpdateCtx(ctx, "UPDATE `shop`.`items` SET `status`=? WHERE `status`=? RETURNING `id`", "closed", "open")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"id":7}]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReturningNoPrimaryKey(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")

	mock.ExpectBegin()
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}))
	mock.ExpectRollback()
	sc := a.UpdateCtx(ctx, "UPDATE `shop`.`items` SET `name`=? WHERE `id`=? RETURNING `id`", "bea", int64(4))
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReturningNoMatch(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")

	mock.ExpectBegin()
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", ""))
	mock.ExpectQuery("SELECT `id` FROM `shop`.`items` WHERE `id`=? FOR UPDATE").
		WithArgs(int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("UPDATE `shop`.`items` SET `name`=? WHERE `id`=?").
		WithArgs("bea", int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	sc := a.UpdateCtx(ctx, "UPDATE `shop`.`items` SET `name`=? WHERE `id`=? RETURNING `id`", "bea", int64(4))
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[]`, string(sc.Bytes()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReturningKeyPredicates(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")

	mock.ExpectBegin()
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", ""))
	mock.ExpectQuery("SELECT `id` FROM `shop`.`items` FOR UPDATE").
		WithoutArgs().
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(4)).AddRow(int64(5)))
	mock.ExpectExec("UPDATE `shop`.`items` SET `name`=?").
		WithArgs("bea").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery("SELECT `id` FROM `shop`.`items` WHERE `id` IN (?,?)").
		WithArgs([]byte("4"), []byte("5")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(4)).AddRow(int64(5)))
	mock.ExpectCommit()
	sc := a.UpdateCtx(ctx, "UPDATE `shop`.`items` SET `name`=? RETURNING `id`", "bea")
	require.NoError(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectBegin()
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "pair").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("a", "").AddRow("b", ""))
	mock.ExpectQuery("SELECT `a`, `b` FROM `shop`.`pair` WHERE `a`=? AND `b`=? FOR UPDATE").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"a", "b"}).AddRow(int64(1), int64(2)).AddRow(int64(3), int64(4)))
	mock.ExpectExec("UPDATE `shop`.`pair` SET `n`=? WHERE `a`=? AND `b`=?").
		WithArgs("x", int64(1), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery("SELECT `a` FROM `shop`.`pair` WHERE (`a`,`b`) IN ((?,?),(?,?))").
		WithArgs([]byte("1"), []byte("2"), []byte("3"), []byte("4")).
		WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow(int64(1)).AddRow(int64(3)))
	mock.ExpectCommit()
	sc = a.UpdateCtx(ctx, "UPDATE `shop`.`pair` SET `n`=? WHERE `a`=? AND `b`=? RETURNING `a`", "x", int64(1), int64(2))
	require.NoError(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateReturningEmptyTable(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")
	mock.ExpectBegin()
	mock.ExpectRollback()
	sc := a.UpdateCtx(ctx, "UPDATE SET `n`=? RETURNING `id`", 1)
	require.Error(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrimaryKeysCacheHit(t *testing.T) {
	a, mock := withMock(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")
	mock.ExpectBegin()
	tx, err := a.GetTransactionCtx(ctx)
	require.NoError(t, err)

	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnError(errors.New("boom"))
	_, err = a.primaryKeys(ctx, tx, "shop", "items")
	require.Error(t, err)

	rows := sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", "auto_increment")
	mock.ExpectQuery(statements.PKColumns).
		WithArgs("shop", "items").
		WillReturnRows(rows)
	first, err := a.primaryKeys(ctx, tx, "shop", "items")
	require.NoError(t, err)
	require.Equal(t, []pkColumn{{name: "id", autoIncrement: true}}, first)

	second, err := a.primaryKeys(ctx, tx, "shop", "items")
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQuoteIdent(t *testing.T) {
	t.Parallel()
	q, err := quoteIdent("a.b")
	require.NoError(t, err)
	require.Equal(t, "`a`.`b`", q)
	_, err = quoteIdent("a b")
	require.Error(t, err)
}
