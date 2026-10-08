package postgres

import (
	"context"
	"net/http"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/prest/prest/v2/adapters"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func batchRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	require.NoError(t, err)
	return req
}

// Records with different keys: columns are the union of all keys and a missing
// key renders DEFAULT, so the column default applies (NOT NULL columns with a
// default keep working) and no value from a later record is dropped.
func TestParseBatchInsertRequest_KeyUnion(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	colsName, placeholders, values, err := adapter.ParseBatchInsertRequest(
		batchRequest(t, `[{"a":"x"},{"b":"y"}]`))
	require.NoError(t, err)
	require.Equal(t, `"a","b"`, colsName)
	require.Equal(t, "($1,DEFAULT),(DEFAULT,$2)", placeholders)
	require.Equal(t, []interface{}{"x", adapters.DefaultValue{}, adapters.DefaultValue{}, "y"}, values)
}

// Uniform records keep the historical output: sequential placeholders, no markers.
func TestParseBatchInsertRequest_Uniform(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	colsName, placeholders, values, err := adapter.ParseBatchInsertRequest(
		batchRequest(t, `[{"name":"a","tags":["t1","t2"]},{"name":"b","tags":["t3"]}]`))
	require.NoError(t, err)
	require.Equal(t, `"name","tags"`, colsName)
	require.Equal(t, "($1,$2),($3,$4)", placeholders)
	require.Equal(t, []interface{}{"a", `{"t1","t2"}`, "b", `{"t3"}`}, values)
	require.False(t, adapters.HasDefaults(values))
}

// A record key that is not a valid identifier is rejected before any SQL is built.
func TestParseBatchInsertRequest_InvalidIdentifier(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	_, _, _, err := adapter.ParseBatchInsertRequest(
		batchRequest(t, `[{"a":1},{"b\"; DROP TABLE x; --":2}]`))
	require.ErrorIs(t, err, ErrInvalidIdentifier)
}

// Empty set and malformed JSON keep returning errors.
func TestParseBatchInsertRequest_Errors(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	_, _, _, err := adapter.ParseBatchInsertRequest(batchRequest(t, `[]`))
	require.ErrorIs(t, err, ErrBodyEmpty)

	_, _, _, err = adapter.ParseBatchInsertRequest(batchRequest(t, `{`))
	require.Error(t, err)
}

// DEFAULT markers are SQL-only: the executor binds just the real values.
func TestBatchInsertValuesCtx_DropsDefaultMarkers(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)
	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	sql := `INSERT INTO "test"."public"."users"("a","b") VALUES($1,DEFAULT),(DEFAULT,$2)`
	ctxMock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("x", "y").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).
			AddRow([]byte(`{"a":"x"}`)).
			AddRow([]byte(`{"b":"y"}`)))

	sc := adapter.BatchInsertValuesCtx(ctx, sql, "x", adapters.DefaultValue{}, adapters.DefaultValue{}, "y")
	require.NoError(t, sc.Err())
	require.JSONEq(t, `[{"a":"x"},{"b":"y"}]`, string(sc.Bytes()))
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

// Non-context variant binds only real values too.
func TestBatchInsertValues_DropsDefaultMarkers(t *testing.T) {
	adapter, mock := withSQLMock(t)
	sql := `INSERT INTO "test"."public"."users"("a","b") VALUES($1,DEFAULT),(DEFAULT,$2)`
	mock.ExpectPrepare(`INSERT INTO "test"."public"."users"`).
		ExpectQuery().
		WithArgs("x", "y").
		WillReturnRows(sqlmock.NewRows([]string{"row_to_json"}).
			AddRow([]byte(`{"a":"x"}`)))

	sc := adapter.BatchInsertValues(sql, "x", adapters.DefaultValue{}, adapters.DefaultValue{}, "y")
	require.NoError(t, sc.Err())
	require.NoError(t, mock.ExpectationsWereMet())
}

// COPY has no DEFAULT keyword: heterogeneous records fail fast, no transaction opened.
func TestBatchInsertCopy_RejectsDefaultMarkers(t *testing.T) {
	adapter, mock := withSQLMock(t)

	sc := adapter.BatchInsertCopy(defaultMockDB, "public", "users", []string{`"a"`, `"b"`},
		"x", adapters.DefaultValue{}, adapters.DefaultValue{}, "y")
	require.EqualError(t, sc.Err(), errCopyRequiresUniformKeys.Error())
	require.NoError(t, mock.ExpectationsWereMet())

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, defaultMockDB)
	sc = adapter.BatchInsertCopyCtx(ctx, defaultMockDB, "public", "users", []string{`"a"`, `"b"`},
		"x", adapters.DefaultValue{}, adapters.DefaultValue{}, "y")
	require.EqualError(t, sc.Err(), "copy batch requires every record to have the same keys")
	require.NoError(t, mock.ExpectationsWereMet())
}
