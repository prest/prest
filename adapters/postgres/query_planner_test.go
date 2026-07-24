package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/prest/prest/v2/adapters/queryplan"
	pctx "github.com/prest/prest/v2/context"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// seqScanPlanJSON is a postgres plan for an unindexed filter over one table.
const seqScanPlanJSON = `[{"Plan":{"Node Type":"Seq Scan","Parallel Aware":false,` +
	`"Relation Name":"orders","Total Cost":1834.25,"Plan Rows":98123,"Plan Width":40}}]`

// stubPlanner records the calls the adapter delegates to the injected planner.
type stubPlanner struct {
	calls int
	ctx   context.Context
	sql   string
}

func (s *stubPlanner) Explain(SQL string, _ ...interface{}) (*queryplan.Node, error) {
	s.calls++
	s.sql = SQL
	return &queryplan.Node{NodeType: "stub"}, nil
}

func (s *stubPlanner) ExplainCtx(ctx context.Context, SQL string, _ ...interface{}) (*queryplan.Node, error) {
	s.ctx = ctx
	return s.Explain(SQL)
}

// The adapter plans against the database named in the context, prefixing the
// statement through the planner it was built with.
func TestExplainCtx_UsesContextDatabase(t *testing.T) {
	t.Parallel()

	adapter, defaultMock, ctxMock := withSQLMocks(t)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	ctxMock.ExpectPrepare(`EXPLAIN \(FORMAT JSON\) SELECT \* FROM orders WHERE status = \$1`).
		ExpectQuery().
		WithArgs("open").
		WillReturnRows(sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow([]byte(seqScanPlanJSON)))

	plan, err := adapter.ExplainCtx(ctx, "SELECT * FROM orders WHERE status = $1", "open")
	require.NoError(t, err)
	require.Equal(t, queryplan.OpSeqScan, plan.Operation)
	require.Equal(t, "orders", plan.Relation)
	require.NoError(t, ctxMock.ExpectationsWereMet())
	require.NoError(t, defaultMock.ExpectationsWereMet())
}

// Explain without a context falls back to the default pooled connection.
func TestExplain_UsesDefaultConnection(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`EXPLAIN \(FORMAT JSON\) SELECT 1`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow([]byte(seqScanPlanJSON)))

	plan, err := adapter.Explain("SELECT 1")
	require.NoError(t, err)
	require.Equal(t, "orders", plan.Relation)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A pool that cannot hand out a connection fails before any statement is
// prepared, so the guard sees a planning error rather than an empty plan.
// Not parallel: withFailingDBConnect swaps a package-level hook.
func TestExplain_ConnectionFailure(t *testing.T) {
	adapter := withFailingDBConnect(t, "connect failed")

	plan, err := adapter.Explain("SELECT 1")
	require.ErrorContains(t, err, "select database for query plan")
	require.ErrorContains(t, err, "connect")
	require.Nil(t, plan)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, contextMockDB)
	plan, err = adapter.ExplainCtx(ctx, "SELECT 1")
	require.ErrorContains(t, err, "connect")
	require.Nil(t, plan)
}

// A statement postgres refuses to prepare (invalid SQL) is reported as is.
func TestExplainCtx_PrepareError(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)

	mock.ExpectPrepare(`EXPLAIN \(FORMAT JSON\)`).WillReturnError(errors.New("syntax error"))

	plan, err := adapter.ExplainCtx(context.Background(), "SELECT FROM")
	require.ErrorContains(t, err, "syntax error")
	require.Nil(t, plan)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ExplainRow is the only capability the planner needs from the adapter: it
// resolves the connection, runs the statement and hands back the raw payload.
func TestExplainRow(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	mock.ExpectPrepare(`EXPLAIN \(FORMAT JSON\) SELECT 1`).
		ExpectQuery().
		WillReturnRows(sqlmock.NewRows([]string{"QUERY PLAN"}).AddRow([]byte(seqScanPlanJSON)))

	raw, err := adapter.ExplainRow(context.Background(), "EXPLAIN (FORMAT JSON) SELECT 1")
	require.NoError(t, err)
	require.JSONEq(t, seqScanPlanJSON, string(raw))
	require.NoError(t, mock.ExpectationsWereMet())
}

// Failures name the step that failed and keep the original error unwrappable.
func TestExplainRow_ErrorsAreWrapped(t *testing.T) {
	t.Parallel()

	adapter, mock := withSQLMock(t)
	prepareErr := errors.New("syntax error")
	mock.ExpectPrepare(`EXPLAIN \(FORMAT JSON\)`).WillReturnError(prepareErr)

	raw, err := adapter.ExplainRow(context.Background(), "EXPLAIN (FORMAT JSON) SELECT FROM")
	require.ErrorIs(t, err, prepareErr)
	require.Contains(t, err.Error(), "prepare query plan statement")
	require.Nil(t, raw)
	require.NoError(t, mock.ExpectationsWereMet())
}

// New installs the postgres planner, and WithQueryPlanner replaces it, so an
// engine with a different EXPLAIN output can be wired without touching the adapter.
func TestWithQueryPlanner(t *testing.T) {
	t.Parallel()

	planner := &stubPlanner{}
	adapter := New(defaultTestConf(), WithQueryPlanner(planner)).(*postgres)

	plan, err := adapter.ExplainCtx(context.Background(), "SELECT 1")
	require.NoError(t, err)
	require.Equal(t, "stub", plan.NodeType)
	require.Equal(t, 1, planner.calls)
	require.Equal(t, "SELECT 1", planner.sql)

	// The default adapter uses the postgres planner instead.
	require.IsType(t, &queryplan.Postgres{}, New(defaultTestConf()).(*postgres).planner)
}

// The postgres adapter must satisfy the optional planner port so Query Guard can
// be enabled on it.
func TestPostgresImplementsQueryPlanner(t *testing.T) {
	t.Parallel()
	require.Implements(t, (*queryplan.Planner)(nil), testAdapter())
}
