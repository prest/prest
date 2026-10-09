package timescaledb

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

// accessorAdapter embeds adapters.Adapter and adds DatabaseAccessor for helper tests.
type accessorAdapter struct {
	adapters.Adapter
	db  *sqlx.DB
	err error
}

func (a *accessorAdapter) DB() (*sqlx.DB, error) {
	return a.db, a.err
}

type connectorAdapter struct {
	adapters.Adapter
	err error
}

func (c *connectorAdapter) Connect() error {
	return c.err
}

type connectAndAccessAdapter struct {
	adapters.Adapter
	connectErr error
	db         *sqlx.DB
	dbErr      error
}

func (a *connectAndAccessAdapter) Connect() error {
	return a.connectErr
}

func (a *connectAndAccessAdapter) DB() (*sqlx.DB, error) {
	return a.db, a.dbErr
}

type pingerAdapter struct {
	adapters.Adapter
	err error
}

func (p *pingerAdapter) Ping(context.Context) error {
	return p.err
}

func newSQLMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mockSQL, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	t.Cleanup(func() { _ = sqlxDB.Close() })
	return sqlxDB, mockSQL
}

func expectTimescaleExists(mockSQL sqlmock.Sqlmock, exists bool) {
	mockSQL.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_extension WHERE extname='timescaledb'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

func Test_verifyTimescaleDB(t *testing.T) {
	dbErr := errors.New("db unavailable")
	queryErr := errors.New("query failed")

	sqlxOK, mockOK := newSQLMock(t)
	expectTimescaleExists(mockOK, true)

	sqlxMissing, mockMissing := newSQLMock(t)
	expectTimescaleExists(mockMissing, false)

	sqlxQueryFail, mockQueryFail := newSQLMock(t)
	mockQueryFail.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_extension WHERE extname='timescaledb'\)`).
		WillReturnError(queryErr)

	base := mock.New(t)

	tests := []struct {
		name    string
		a       adapters.Adapter
		wantErr error
		errText string
	}{
		{
			name:    "not a DatabaseAccessor",
			a:       base,
			wantErr: ErrNotTimescaleDBAdapter,
		},
		{
			name: "DB returns error",
			a: &accessorAdapter{
				Adapter: base,
				err:     dbErr,
			},
			wantErr: dbErr,
		},
		{
			name: "extension query fails",
			a: &accessorAdapter{
				Adapter: base,
				db:      sqlxQueryFail,
			},
			wantErr: queryErr,
		},
		{
			name: "extension not installed",
			a: &accessorAdapter{
				Adapter: base,
				db:      sqlxMissing,
			},
			errText: "timescaledb extension not found",
		},
		{
			name: "extension present",
			a: &accessorAdapter{
				Adapter: base,
				db:      sqlxOK,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := verifyTimescaleDB(tt.a)
			if tt.wantErr != nil {
				require.ErrorIs(t, gotErr, tt.wantErr)
				return
			}
			if tt.errText != "" {
				require.Error(t, gotErr)
				require.Contains(t, gotErr.Error(), tt.errText)
				return
			}
			require.NoError(t, gotErr)
		})
	}

	require.NoError(t, mockOK.ExpectationsWereMet())
	require.NoError(t, mockMissing.ExpectationsWereMet())
	require.NoError(t, mockQueryFail.ExpectationsWereMet())
}

func TestAdapter_Connect(t *testing.T) {
	base := mock.New(t)
	connectErr := errors.New("connect failed")

	t.Run("embedded is not DatabaseConnector", func(t *testing.T) {
		a := &Adapter{Adapter: base}
		require.ErrorIs(t, a.Connect(), ErrNotTimescaleDBAdapter)
	})

	t.Run("delegates connect error", func(t *testing.T) {
		a := &Adapter{Adapter: &connectorAdapter{Adapter: base, err: connectErr}}
		require.ErrorIs(t, a.Connect(), connectErr)
	})

	t.Run("delegates connect success", func(t *testing.T) {
		a := &Adapter{Adapter: &connectorAdapter{Adapter: base}}
		require.NoError(t, a.Connect())
	})
}

func TestAdapter_DB(t *testing.T) {
	base := mock.New(t)
	dbErr := errors.New("db failed")
	sqlxDB, _ := newSQLMock(t)

	t.Run("embedded is not DatabaseAccessor", func(t *testing.T) {
		a := &Adapter{Adapter: base}
		got, err := a.DB()
		require.Nil(t, got)
		require.ErrorIs(t, err, ErrNotTimescaleDBAdapter)
	})

	t.Run("delegates DB error", func(t *testing.T) {
		a := &Adapter{Adapter: &accessorAdapter{Adapter: base, err: dbErr}}
		got, err := a.DB()
		require.Nil(t, got)
		require.ErrorIs(t, err, dbErr)
	})

	t.Run("delegates DB success", func(t *testing.T) {
		a := &Adapter{Adapter: &accessorAdapter{Adapter: base, db: sqlxDB}}
		got, err := a.DB()
		require.NoError(t, err)
		require.Same(t, sqlxDB, got)
	})
}

func TestConnect(t *testing.T) {
	base := mock.New(t)
	connectErr := errors.New("connect failed")

	sqlxOK, mockOK := newSQLMock(t)
	expectTimescaleExists(mockOK, true)

	t.Run("not a DatabaseConnector", func(t *testing.T) {
		require.ErrorIs(t, Connect(base), ErrNotTimescaleDBAdapter)
	})

	t.Run("connect fails", func(t *testing.T) {
		a := &connectAndAccessAdapter{Adapter: base, connectErr: connectErr}
		require.ErrorIs(t, Connect(a), connectErr)
	})

	t.Run("connect ok then verify fails without accessor", func(t *testing.T) {
		a := &connectorAdapter{Adapter: base}
		require.ErrorIs(t, Connect(a), ErrNotTimescaleDBAdapter)
	})

	t.Run("connect and verify succeed", func(t *testing.T) {
		a := &connectAndAccessAdapter{Adapter: base, db: sqlxOK}
		require.NoError(t, Connect(a))
		require.NoError(t, mockOK.ExpectationsWereMet())
	})
}

func TestIsTimescaleDB(t *testing.T) {
	base := mock.New(t)
	dbErr := errors.New("db unavailable")
	queryErr := errors.New("query failed")

	sqlxTrue, mockTrue := newSQLMock(t)
	expectTimescaleExists(mockTrue, true)

	sqlxFalse, mockFalse := newSQLMock(t)
	expectTimescaleExists(mockFalse, false)

	sqlxQueryFail, mockQueryFail := newSQLMock(t)
	mockQueryFail.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pg_extension WHERE extname='timescaledb'\)`).
		WillReturnError(queryErr)

	t.Run("not a DatabaseAccessor", func(t *testing.T) {
		ok, err := IsTimescaleDB(base)
		require.False(t, ok)
		require.ErrorIs(t, err, ErrNotTimescaleDBAdapter)
	})

	t.Run("DB error", func(t *testing.T) {
		ok, err := IsTimescaleDB(&accessorAdapter{Adapter: base, err: dbErr})
		require.False(t, ok)
		require.ErrorIs(t, err, dbErr)
	})

	t.Run("query error", func(t *testing.T) {
		ok, err := IsTimescaleDB(&accessorAdapter{Adapter: base, db: sqlxQueryFail})
		require.False(t, ok)
		require.ErrorIs(t, err, queryErr)
	})

	t.Run("extension missing", func(t *testing.T) {
		ok, err := IsTimescaleDB(&accessorAdapter{Adapter: base, db: sqlxFalse})
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("extension present", func(t *testing.T) {
		ok, err := IsTimescaleDB(&accessorAdapter{Adapter: base, db: sqlxTrue})
		require.NoError(t, err)
		require.True(t, ok)
	})

	require.NoError(t, mockTrue.ExpectationsWereMet())
	require.NoError(t, mockFalse.ExpectationsWereMet())
	require.NoError(t, mockQueryFail.ExpectationsWereMet())
}

func TestClose(t *testing.T) {
	// postgres.Close is a no-op for non-*postgres adapters; exercise the helper without a concrete postgres type.
	require.NotPanics(t, func() {
		Close(mock.New(t))
	})
}

func TestDB(t *testing.T) {
	base := mock.New(t)
	sqlxDB, _ := newSQLMock(t)

	t.Run("not a DatabaseAccessor", func(t *testing.T) {
		got, err := DB(base)
		require.Nil(t, got)
		require.ErrorIs(t, err, postgres.ErrNotPostgresAdapter)
	})

	t.Run("success", func(t *testing.T) {
		got, err := DB(&accessorAdapter{Adapter: base, db: sqlxDB})
		require.NoError(t, err)
		require.Same(t, sqlxDB, got)
	})
}

func TestPing(t *testing.T) {
	base := mock.New(t)
	pingErr := errors.New("ping failed")

	t.Run("success", func(t *testing.T) {
		require.NoError(t, Ping(context.Background(), &pingerAdapter{Adapter: base}))
	})

	t.Run("error", func(t *testing.T) {
		require.ErrorIs(t, Ping(context.Background(), &pingerAdapter{Adapter: base, err: pingErr}), pingErr)
	})
}

var (
	_ adapters.QueryRegistry            = (*Adapter)(nil)
	_ adapters.ScriptPermissionsChecker = (*Adapter)(nil)
)

// registryAdapter embeds adapters.Adapter and records QueryRegistry and
// ScriptPermissions calls so forwarding can be asserted.
type registryAdapter struct {
	adapters.Adapter
	gotAlias, gotLocation, gotName, gotOp, gotUser, gotPath, gotPolicy string
	gotQuery                                                           adapters.StoredQuery
	list                                                               []adapters.StoredQuery
	query                                                              adapters.StoredQuery
	report                                                             adapters.ImportReport
	allow                                                              bool
	err                                                                error
}

func (r *registryAdapter) ListQueries(_ context.Context, alias, location string) ([]adapters.StoredQuery, error) {
	r.gotAlias, r.gotLocation = alias, location
	return r.list, r.err
}

func (r *registryAdapter) GetQuery(_ context.Context, alias, location, name string) (adapters.StoredQuery, error) {
	r.gotAlias, r.gotLocation, r.gotName = alias, location, name
	return r.query, r.err
}

func (r *registryAdapter) UpsertQuery(_ context.Context, q adapters.StoredQuery) error {
	r.gotQuery = q
	return r.err
}

func (r *registryAdapter) DeleteQuery(_ context.Context, alias, location, name string) error {
	r.gotAlias, r.gotLocation, r.gotName = alias, location, name
	return r.err
}

func (r *registryAdapter) ImportFromFilesystem(_ context.Context, path, policy string) (adapters.ImportReport, error) {
	r.gotPath, r.gotPolicy = path, policy
	return r.report, r.err
}

func (r *registryAdapter) ScriptPermissions(_ context.Context, alias, location, name, op, user string) bool {
	r.gotAlias, r.gotLocation, r.gotName, r.gotOp, r.gotUser = alias, location, name, op, user
	return r.allow
}

func TestQueryRegistryForwards(t *testing.T) {
	ctx := context.Background()
	stored := adapters.StoredQuery{DatabaseAlias: "prest-test", Location: "itest", Name: "q", ReadSQL: "SELECT 1"}
	stub := &registryAdapter{
		list:   []adapters.StoredQuery{stored},
		query:  stored,
		report: adapters.ImportReport{Inserted: 2, Updated: 1, Skipped: 3},
	}
	a := &Adapter{Adapter: stub}

	list, err := a.ListQueries(ctx, "prest-test", "itest")
	require.NoError(t, err)
	require.Equal(t, stub.list, list)
	require.Equal(t, []string{"prest-test", "itest"}, []string{stub.gotAlias, stub.gotLocation})

	got, err := a.GetQuery(ctx, "prest-test", "itest", "q")
	require.NoError(t, err)
	require.Equal(t, stored, got)
	require.Equal(t, "q", stub.gotName)

	require.NoError(t, a.UpsertQuery(ctx, stored))
	require.Equal(t, stored, stub.gotQuery)

	require.NoError(t, a.DeleteQuery(ctx, "other", "loc", "gone"))
	require.Equal(t, []string{"other", "loc", "gone"}, []string{stub.gotAlias, stub.gotLocation, stub.gotName})

	report, err := a.ImportFromFilesystem(ctx, "/queries", "update")
	require.NoError(t, err)
	require.Equal(t, stub.report, report)
	require.Equal(t, []string{"/queries", "update"}, []string{stub.gotPath, stub.gotPolicy})
}

func TestQueryRegistryForwardsErrors(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	a := &Adapter{Adapter: &registryAdapter{err: boom}}

	_, err := a.ListQueries(ctx, "", "")
	require.ErrorIs(t, err, boom)
	_, err = a.GetQuery(ctx, "", "l", "n")
	require.ErrorIs(t, err, boom)
	require.ErrorIs(t, a.UpsertQuery(ctx, adapters.StoredQuery{}), boom)
	require.ErrorIs(t, a.DeleteQuery(ctx, "", "l", "n"), boom)
	_, err = a.ImportFromFilesystem(ctx, "/q", "skip")
	require.ErrorIs(t, err, boom)
}

func TestScriptPermissionsForwards(t *testing.T) {
	stub := &registryAdapter{allow: true}
	a := &Adapter{Adapter: stub}

	require.True(t, a.ScriptPermissions(context.Background(), "prest-test", "itest", "q", "read", "ada"))
	require.Equal(t,
		[]string{"prest-test", "itest", "q", "read", "ada"},
		[]string{stub.gotAlias, stub.gotLocation, stub.gotName, stub.gotOp, stub.gotUser})

	stub.allow = false
	require.False(t, a.ScriptPermissions(context.Background(), "prest-test", "itest", "q", "write", "ada"))
}

func TestQueryRegistryUnsupported(t *testing.T) {
	// The mock adapter implements neither port, so the wrapper must refuse
	// instead of pretending the registry exists.
	inner := mock.New(t)
	_, isRegistry := interface{}(inner).(adapters.QueryRegistry)
	_, isChecker := interface{}(inner).(adapters.ScriptPermissionsChecker)
	require.False(t, isRegistry)
	require.False(t, isChecker)

	ctx := context.Background()
	a := &Adapter{Adapter: inner}
	_, err := a.ListQueries(ctx, "", "")
	require.ErrorIs(t, err, ErrNotTimescaleDBAdapter)
	_, err = a.GetQuery(ctx, "", "l", "n")
	require.ErrorIs(t, err, ErrNotTimescaleDBAdapter)
	require.ErrorIs(t, a.UpsertQuery(ctx, adapters.StoredQuery{}), ErrNotTimescaleDBAdapter)
	require.ErrorIs(t, a.DeleteQuery(ctx, "", "l", "n"), ErrNotTimescaleDBAdapter)
	_, err = a.ImportFromFilesystem(ctx, "/q", "skip")
	require.ErrorIs(t, err, ErrNotTimescaleDBAdapter)
	require.False(t, a.ScriptPermissions(ctx, "", "l", "n", "read", ""))
}

var _ adapters.Dialect = (*Adapter)(nil)

// The wrapped postgres adapter's Dialect is forwarded, so auth and MCP keep
// working on TimescaleDB now that Dialect is no longer part of adapters.Adapter.
func TestDialectForwards(t *testing.T) {
	t.Parallel()

	a := &Adapter{Adapter: postgres.New(&config.Prest{})}
	quoted, err := a.QuoteIdentifier("a.b")
	require.NoError(t, err)
	require.Equal(t, `"a"."b"`, quoted)
	require.Equal(t, "$2", a.Placeholder(2))
}

// A wrapped adapter without Dialect cannot quote, but placeholders stay
// Postgres-style because TimescaleDB is wire-compatible.
func TestDialectUnsupported(t *testing.T) {
	t.Parallel()

	a := &Adapter{Adapter: struct{ adapters.Adapter }{}}
	_, err := a.QuoteIdentifier("a")
	require.ErrorIs(t, err, ErrNotTimescaleDBAdapter)
	require.Equal(t, "$1", a.Placeholder(1))
}
