package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func withFlex(t *testing.T) (*Adapter, sqlmock.Sqlmock) {
	t.Helper()
	raw, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		if strings.TrimSpace(expected) == strings.TrimSpace(actual) || strings.Contains(actual, expected) {
			return nil
		}
		return fmt.Errorf("actual %q does not contain %q", actual, expected)
	})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	a := New(testCfg()).(*Adapter)
	a.SetDatabase("shop")
	require.NoError(t, a.conn.InjectDBForTest(sqlx.NewDb(raw, "sqlmock")))
	return a, mock
}

func bodyReq(raw, body string) *http.Request {
	r := req(raw)
	r.Body = io.NopCloser(strings.NewReader(body))
	return r
}

func TestOperatorsBuildersAndErrors(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)

	cases := []struct{ raw, want string }{
		{"/t?n=$ne.1", "`n` != ?"},
		{"/t?n=$gt.1", "`n` > ?"},
		{"/t?n=$gte.1", "`n` >= ?"},
		{"/t?n=$lt.1", "`n` < ?"},
		{"/t?n=$lte.1", "`n` <= ?"},
		{"/t?n=$nin.1,2", "`n` NOT IN (?,?)"},
		{"/t?n=$some.1,2", "`n` IN (?,?)"},
		{"/t?n=$like.a", "`n` LIKE ?"},
		{"/t?n=$nlike.a", "`n` NOT LIKE ?"},
		{"/t?n=$nilike.a", "`n` NOT LIKE ?"},
		{"/t?n=$null.", "`n` IS NULL"},
		{"/t?n=$notnull.", "`n` IS NOT NULL"},
		{"/t?n=$true.", "`n` IS TRUE"},
		{"/t?n=$nottrue.", "`n` IS NOT TRUE"},
		{"/t?n=$false.", "`n` IS FALSE"},
		{"/t?n=$notfalse.", "`n` IS NOT FALSE"},
	}
	for _, tc := range cases {
		where, _, err := a.WhereByRequest(req(tc.raw), 1)
		require.NoError(t, err, tc.raw)
		require.Equal(t, tc.want, where, tc.raw)
	}

	_, _, err := a.WhereByRequest(req("/t?n=$nope.1"), 1)
	require.ErrorIs(t, err, errInvalidOperator)
	_, _, err = a.WhereByRequest(req("/t?n="), 1)
	require.Error(t, err)
	_, _, err = a.WhereByRequest(req("/t?1bad=$eq.1"), 1)
	require.Error(t, err)
	_, _, err = a.WhereByRequest(req("/t?meta->>x:nope=$eq.1"), 1)
	require.Error(t, err)
	_, _, err = a.WhereByRequest(req("/t?1bad->>x:jsonb=$eq.1"), 1)
	require.Error(t, err)
	_, _, err = a.WhereByRequest(req("/t?col:tsquery=$eq.a"), 1)
	require.ErrorIs(t, err, errUnsupported)

	where, vals, err := a.WhereByRequest(req("/t?_or=name=$eq.ada||id=$gt.1 OR nick=$eq.'o''brien' OR title=$eq.\"a\"\"b\""), 1)
	require.NoError(t, err)
	require.Contains(t, where, " OR ")
	require.Contains(t, vals, "ada")
	require.Contains(t, vals, "1")

	ret, err := a.ReturningByRequest(req("/t?_returning=id&_returning=*"))
	require.NoError(t, err)
	require.Equal(t, "`id`, *", ret)
	_, err = a.ReturningByRequest(req("/t?_returning=a%20b"))
	require.Error(t, err)

	set, setVals, err := a.SetByRequest(bodyReq("/t", `{"b":"z","a":1}`), 1)
	require.NoError(t, err)
	require.Equal(t, "`a`=?, `b`=?", set)
	require.Equal(t, []any{float64(1), "z"}, setVals)
	_, _, err = a.SetByRequest(bodyReq("/t", `{}`), 1)
	require.ErrorIs(t, err, errBodyEmpty)
	_, _, err = a.SetByRequest(bodyReq("/t", `{"bad name":1}`), 1)
	require.Error(t, err)

	names, ph, bvals, err := a.ParseBatchInsertRequest(bodyReq("/t", `[{"b":"z","a":1},{"a":2,"b":"y"}]`))
	require.NoError(t, err)
	require.Equal(t, "`a`,`b`", names)
	require.Contains(t, ph, "?")
	require.Len(t, bvals, 4)
	_, _, _, err = a.ParseBatchInsertRequest(bodyReq("/t", `[]`))
	require.ErrorIs(t, err, errBodyEmpty)

	require.Contains(t, a.UpdateSQL("a", "shop", "items", "`name`=?"), "UPDATE `shop`.`items` SET")
	require.Equal(t, "DELETE FROM `shop`.`items`", a.DeleteSQL("a", "shop", "items"))
	_, err = a.SelectFields(nil)
	require.ErrorIs(t, err, errMustSelectOneField)
	_, err = a.SelectFields([]string{"nope:price"})
	require.Error(t, err)
	sel, err := a.SelectFields([]string{"*", "items.*", "sum:price:total", "SUM(`name`)"})
	require.NoError(t, err)
	require.Contains(t, sel, "*")
	require.Contains(t, sel, "`items`.*")

	d, err := a.DistinctClause(req("/t?_distinct=true"))
	require.NoError(t, err)
	require.Equal(t, "SELECT DISTINCT", d)
	d, err = a.DistinctClause(req("/t"))
	require.NoError(t, err)
	require.Empty(t, d)

	count, err := a.CountByRequest(req("/t?_count=id,*&_select=name"))
	require.NoError(t, err)
	require.Contains(t, count, "COUNT(`id`,*)")
	require.Contains(t, count, "`name`")
	_, err = a.CountByRequest(req("/t?_count=bad name"))
	require.Error(t, err)
	empty, err := a.CountByRequest(req("/t"))
	require.NoError(t, err)
	require.Empty(t, empty)

	_, err = a.PaginateIfPossible(req("/t?_page=nope"))
	require.Error(t, err)
	_, err = a.PaginateIfPossible(req("/t?_page=1&_page_size=nope"))
	require.Error(t, err)
	page, err := a.PaginateIfPossible(req("/t?_page=1"))
	require.NoError(t, err)
	require.Contains(t, page, "LIMIT")

	require.Empty(t, a.GroupByClause(req("/t")))
	require.Contains(t, a.GroupByClause(req("/t?_groupby=upper(name)")), "upper(name)")
	require.Empty(t, a.GroupByClause(req("/t?_groupby=pg_sleep(1)")))
	require.Contains(t, a.GroupByClause(req("/t?_groupby=status->>having:avg:age:$gt:18")), "HAVING AVG(`age`) > 18")
	require.Contains(t, a.GroupByClause(req("/t?_groupby=status->>having:avg:age:$gt:o'brien")), "HAVING")

	joins, err := a.JoinByRequest(req("/t?_join=LEFT:shop.orders:orders.id:$eq:items.id&_join=CROSS:tags:tags.id:$ne:items.id"))
	require.NoError(t, err)
	require.Len(t, joins, 2)
	_, err = a.JoinByRequest(req("/t?_join=SIDE:orders:orders.id:$eq:items.id"))
	require.Error(t, err)
	_, err = a.JoinByRequest(req("/t?_join=INNER:orders:orders.id:$eq:items.id&_join=LEFT:orders:orders.id:$eq:items.id"))
	require.Error(t, err)

	require.Equal(t, " ORDER BY custom", a.SchemaOrderBy(" ORDER BY custom", false))
	require.Equal(t, " ORDER BY schema_name ASC", a.SchemaOrderBy("", false))
	require.Empty(t, a.SchemaOrderBy("", true))
	_, counted := a.SchemaClause(req("/schemas?_count=schema_name"))
	require.True(t, counted)
	require.Equal(t, " ORDER BY `name`", a.TableOrderBy(" ORDER BY `name`"))
	require.Contains(t, a.TableOrderBy(""), "`schema`")
	require.Equal(t, " ORDER BY id", a.SchemaTablesOrderBy(" ORDER BY id"))
	require.Contains(t, a.SchemaTablesOrderBy(""), "`name`")
	require.Equal(t, "WHERE 1=1", a.TableWhere(""))
	require.Contains(t, a.SchemaTablesWhere(""), "TABLE_SCHEMA = ?")

	_, err = a.OrderByRequest(req("/t?_order=bad name"))
	require.Error(t, err)
	emptyOrder, err := a.OrderByRequest(req("/t"))
	require.NoError(t, err)
	require.Empty(t, emptyOrder)
}

func TestJSONScanTypes(t *testing.T) {
	a, mock := withFlex(t)
	mock.ExpectQuery("SELECT kinds").
		WillReturnRows(sqlmock.NewRows([]string{"n", "f", "b", "arr", "dec"}).
			AddRow(int64(2), 1.5, true, []byte(`[1,2]`), []byte("9.5")))
	sc := a.QueryCtx(context.Background(), "SELECT kinds")
	require.NoError(t, sc.Err())
	body := string(sc.Bytes())
	require.Contains(t, body, `"f":1.5`)
	require.Contains(t, body, `"b":true`)
	require.Contains(t, body, `[1,2]`)
	require.Contains(t, body, `"9.5"`)
}

func TestWriteWrappersAndScripts(t *testing.T) {
	a, mock := withFlex(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `shop`.`items`(`name`) VALUES(?)").
		WithArgs("ada").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("COLUMN_KEY").
		WithArgs("shop", "items").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("sku", ""))
	mock.ExpectCommit()
	sc := a.Insert("INSERT INTO `shop`.`items`(`name`) VALUES(?)", "ada")
	require.NoError(t, sc.Err())
	require.Contains(t, string(sc.Bytes()), "ada")

	mock.ExpectBegin()
	tx, err := a.GetTransaction()
	require.NoError(t, err)
	mock.ExpectExec("INSERT INTO `shop`.`notes`(`id`) VALUES(?)").
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("COLUMN_KEY").
		WithArgs("shop", "notes").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", ""))
	mock.ExpectQuery("SELECT * FROM").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	sc = a.InsertWithTransaction(tx, "INSERT INTO `shop`.`notes`(`id`) VALUES(?)", int64(9))
	require.NoError(t, sc.Err())

	mock.ExpectExec("UPDATE `shop`.`items` SET `name`=?").
		WillReturnResult(sqlmock.NewResult(0, 2))
	sc = a.Update("UPDATE `shop`.`items` SET `name`=?", "z")
	require.NoError(t, sc.Err())
	require.Contains(t, string(sc.Bytes()), `"rows_affected":2`)

	mock.ExpectExec("DELETE FROM `shop`.`items`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	sc = a.Delete("DELETE FROM `shop`.`items`")
	require.NoError(t, sc.Err())

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `shop`.`batch`(`name`) VALUES(?),(?)").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectQuery("COLUMN_KEY").
		WithArgs("shop", "batch").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).AddRow("id", "auto_increment"))
	mock.ExpectQuery("SELECT * FROM").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)).AddRow(int64(2)))
	mock.ExpectCommit()
	sc = a.BatchInsertValuesCtx(ctx, "INSERT INTO `shop`.`batch`(`name`) VALUES(?),(?)", "a", "b")
	require.NoError(t, sc.Err())
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO nope").WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()
	sc = a.BatchInsertValuesCtx(ctx, "INSERT INTO nope", "a")
	require.NoError(t, sc.Err())
	require.Contains(t, string(sc.Bytes()), "rows_affected")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `shop`.`pair`(`name`) VALUES(?)").
		WithArgs("a").
		WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectQuery("COLUMN_KEY").
		WithArgs("shop", "pair").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA"}).
			AddRow("id", "auto_increment").
			AddRow("sku", ""))
	mock.ExpectCommit()
	sc = a.BatchInsertValuesCtx(ctx, "INSERT INTO `shop`.`pair`(`name`) VALUES(?)", "a")
	require.NoError(t, sc.Err())
	require.Contains(t, string(sc.Bytes()), "last_insert_id")
	sc = a.BatchInsertCopy("a", "shop", "items", nil)
	require.Error(t, sc.Err())
	sc = a.BatchInsertCopy("a", "shop", "items", []string{"bad name"}, "a")
	require.Error(t, sc.Err())
	sc = a.BatchInsertCopy("a", "shop", "items", []string{"name", "id"}, "a")
	require.Error(t, sc.Err())

	mock.ExpectQuery("information_schema.columns").
		WithArgs("items", "shop").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}).AddRow("id"))
	sc = a.ShowTable("shop", "items")
	require.NoError(t, sc.Err())
	mock.ExpectQuery("information_schema.columns").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	sc = a.ShowTableCtx(ctx, "shop", "items")
	require.NoError(t, sc.Err())
	mock.ExpectQuery("TABLE_SCHEMA NOT IN").
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}))
	sc = a.ShowColumnsCtx(ctx)
	require.NoError(t, sc.Err())

	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(int64(1)))
	sc = a.ExecuteScripts("GET", "SELECT 1", nil)
	require.NoError(t, sc.Err())
	mock.ExpectExec("UPDATE `shop`.`items` SET `n`=1").WillReturnResult(sqlmock.NewResult(0, 1))
	sc = a.ExecuteScriptsCtx(ctx, "PATCH", "UPDATE `shop`.`items` SET `n`=1", nil)
	require.NoError(t, sc.Err())
	sc = a.ExecuteScripts("NOPE", "SELECT 1", nil)
	require.Error(t, sc.Err())

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS `shop`.`prest_users`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, a.EnsureAuthTable(ctx))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS `shop`.`prest_queries`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX").
		WillReturnError(&mysql.MySQLError{Number: 1061, Message: "Duplicate key name"})
	require.NoError(t, a.EnsureQueriesTable(ctx))
}

func TestQueryRegistryAndScripts(t *testing.T) {
	a, mock := withFlex(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "shop")

	cols := []string{"id", "database_alias", "location", "name", "read_sql", "write_sql", "update_sql", "delete_sql", "description", "created_by", "created_at", "updated_at"}
	mock.ExpectQuery("CAST(created_at AS CHAR)").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), "shop", "r", "q", "select 1", nil, nil, nil, nil, nil, "t", "t"))
	list, err := a.ListQueries(ctx, "shop", "r")
	require.NoError(t, err)
	require.Equal(t, "q", list[0].Name)
	require.Equal(t, "select 1", list[0].ReadSQL)

	mock.ExpectQuery("name = ?").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), "shop", "r", "q", "select 1", nil, nil, nil, nil, nil, "t", "t"))
	got, err := a.GetQuery(ctx, "shop", "r", "q")
	require.NoError(t, err)
	require.Equal(t, "select 1", got.ReadSQL)

	require.Error(t, a.UpsertQuery(ctx, adapters.StoredQuery{Location: "bad/loc", Name: "q", ReadSQL: "s"}))
	require.Error(t, a.UpsertQuery(ctx, adapters.StoredQuery{Location: "r", Name: "q"}))
	mock.ExpectExec("ON DUPLICATE KEY UPDATE").WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, a.UpsertQuery(ctx, adapters.StoredQuery{DatabaseAlias: "shop", Location: "r", Name: "q", ReadSQL: "select 1"}))

	mock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, a.DeleteQuery(ctx, "shop", "r", "q"))
	mock.ExpectExec("DELETE FROM").WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, a.DeleteQuery(ctx, "shop", "r", "missing"))

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "reports"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "reports", "items.read.sql"), []byte("select 1"), 0o644))
	mock.ExpectQuery("name = ?").WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectExec("ON DUPLICATE KEY UPDATE").WillReturnResult(sqlmock.NewResult(1, 1))
	report, err := a.ImportFromFilesystem(ctx, dir, config.QueriesImportPolicyError)
	require.NoError(t, err)
	require.Equal(t, 1, report.Inserted)
	_, err = a.ImportFromFilesystem(ctx, filepath.Join(dir, "missing"), "")
	require.NoError(t, err)

	conflictDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(conflictDir, "reports"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(conflictDir, "reports", "items.read.sql"), []byte("select 9"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(conflictDir, "reports", "items.write.sql"), []byte("insert 1"), 0o644))
	existing := sqlmock.NewRows(cols).AddRow(int64(1), "", "reports", "items", "select 1", "old", nil, nil, nil, nil, "t", "t")
	mock.ExpectQuery("name = ?").WillReturnRows(existing)
	_, err = a.ImportFromFilesystem(ctx, conflictDir, config.QueriesImportPolicyError)
	require.Error(t, err)
	mock.ExpectQuery("name = ?").WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), "", "reports", "items", "select 1", "", nil, nil, nil, nil, "t", "t"))
	report, err = a.ImportFromFilesystem(ctx, conflictDir, config.QueriesImportPolicySkip)
	require.NoError(t, err)
	require.Equal(t, 1, report.Skipped)
	mock.ExpectQuery("name = ?").WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), "", "reports", "items", "select 1", "", nil, nil, nil, nil, "t", "t"))
	mock.ExpectExec("ON DUPLICATE KEY UPDATE").WillReturnResult(sqlmock.NewResult(1, 1))
	report, err = a.ImportFromFilesystem(ctx, conflictDir, "")
	require.NoError(t, err)
	require.Equal(t, 1, report.Updated)

	sameDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(sameDir, "reports"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sameDir, "reports", "items.read.sql"), []byte("select 1"), 0o644))
	mock.ExpectQuery("name = ?").WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), "", "reports", "items", "select 1", "", nil, nil, nil, nil, "t", "t"))
	report, err = a.ImportFromFilesystem(ctx, sameDir, "")
	require.NoError(t, err)
	require.Equal(t, 1, report.Skipped)

	mock.ExpectQuery("name = ?").WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectQuery("name = ?").WillReturnRows(sqlmock.NewRows(cols))
	_, err = a.GetQuery(ctx, "shop", "r", "missing")
	require.Error(t, err)

	a.cfg.QueriesPath = dir
	src, err := a.ResolveScript(ctx, "GET", "reports", "items", "shop")
	require.NoError(t, err)
	require.Contains(t, src.Content, "select 1")
	_, err = a.ResolveScript(ctx, "NOPE", "reports", "items", "shop")
	require.Error(t, err)
	_, err = a.GetScript("GET", "../secret", "items")
	require.Error(t, err)
	path, err := a.GetScript("GET", "reports", "items")
	require.NoError(t, err)
	sqlText, _, err := a.ParseScript(path, map[string]any{})
	require.NoError(t, err)
	require.Contains(t, sqlText, "select 1")

	a.cfg.QueriesConf.Storage = config.QueriesStorageDatabase
	mock.ExpectQuery("read_sql").WillReturnRows(sqlmock.NewRows([]string{"read_sql"}).AddRow("select 2"))
	src, err = a.ResolveScript(ctx, "GET", "reports", "items", "shop")
	require.NoError(t, err)
	require.Equal(t, "select 2", src.Content)
}

func TestPermissionsJoinsAndConnect(t *testing.T) {
	cfg := testCfg()
	cfg.AccessConf.Restrict = true
	cfg.AccessConf.Tables = []config.TablesConf{
		{Name: "items", Permissions: []string{"read"}, Fields: []string{"id"}},
		{Name: "orders", Permissions: []string{"read"}, Fields: []string{"*"}},
	}
	a := New(cfg).(*Adapter)
	fields, err := a.FieldsPermissions(req("/t?_select=id,orders.total,secret&_join=INNER:orders:orders.id:$eq:items.id"), "shop", "shop", "items", "read", "")
	require.NoError(t, err)
	require.Contains(t, fields, "items.id")
	require.Contains(t, fields, "orders.total")
	fields, err = a.FieldsPermissions(req("/t?_join=INNER:orders:orders.id:$eq:items.id"), "shop", "shop", "items", "read", "")
	require.NoError(t, err)
	require.NotEmpty(t, fields)
	fields, err = a.FieldsPermissions(req("/t"), "shop", "shop", "items", "delete", "")
	require.NoError(t, err)
	require.Equal(t, []string{"*"}, fields)
	_, err = a.FieldsPermissions(req("/t?_join=INNER:items:items.id:$eq:items.id"), "shop", "shop", "items", "read", "")
	require.Error(t, err)

	raw, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	mock.ExpectPing()
	mock.ExpectPing()
	mock.ExpectPing()
	restore := SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return sqlx.NewDb(raw, "sqlmock"), nil
	})
	t.Cleanup(restore)
	connectCfg := testCfg()
	connectCfg.PGMaxIdleConn = 2
	connectCfg.PGMaxOpenConn = 2
	connected := New(connectCfg).(*Adapter)
	require.NoError(t, connected.Connect())
	require.NoError(t, Connect(connected))
	_, err = connected.DB()
	require.NoError(t, err)
	require.NoError(t, connected.PingAll(context.Background()))
	require.Error(t, Connect(struct{ adapters.Adapter }{}))

	reg := testCfg()
	reg.Databases = []config.DatabaseConf{{Alias: "shop", Engine: config.EngineMySQL}}
	registered := New(reg).(*Adapter)
	registered.SetDatabase("shop")
	require.True(t, registered.IsRegistered("shop"))
	require.False(t, registered.IsRegistered("other"))
	require.Equal(t, "shop", registered.PhysicalName("other"))
	empty := New(&config.Prest{}).(*Adapter)
	require.Equal(t, "x", empty.PhysicalName("x"))
	require.Empty(t, empty.Aliases())
	require.True(t, empty.IsRegistered("anything"))

	var n sql.NullString
	require.False(t, n.Valid)
}

func TestTransactionWrappers(t *testing.T) {
	a, mock := withFlex(t)
	mock.ExpectBegin()
	tx, err := a.GetTransactionCtx(context.Background())
	require.NoError(t, err)
	mock.ExpectExec("UPDATE `shop`.`items` SET `name`=?").WillReturnResult(sqlmock.NewResult(0, 1))
	sc := a.UpdateWithTransaction(tx, "UPDATE `shop`.`items` SET `name`=?", "z")
	require.NoError(t, sc.Err())
	mock.ExpectExec("DELETE FROM `shop`.`items`").WillReturnResult(sqlmock.NewResult(0, 1))
	sc = a.DeleteWithTransaction(tx, "DELETE FROM `shop`.`items`")
	require.NoError(t, sc.Err())
	mock.ExpectQuery("SELECT `id` FROM").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectExec("DELETE FROM `shop`.`items` WHERE `id`=?").WillReturnResult(sqlmock.NewResult(0, 1))
	sc = a.DeleteWithTransaction(tx, "DELETE FROM `shop`.`items` WHERE `id`=? RETURNING `id`", int64(1))
	require.NoError(t, sc.Err())
}
