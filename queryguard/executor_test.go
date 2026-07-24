package queryguard

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/adapters/queryplan"
	"github.com/prest/prest/v2/adapters/scanner"
	pctx "github.com/prest/prest/v2/context"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// plannerExecutor is a mock adapter that also satisfies queryplan.Planner,
// so the guard can be exercised without a database.
type plannerExecutor struct {
	*mockgen.MockAdapter

	plan      *queryplan.Node
	err       error
	calls     int
	lastSQL   string
	lastCtx   context.Context
	lastParam []interface{}
}

func (p *plannerExecutor) Explain(SQL string, params ...interface{}) (*queryplan.Node, error) {
	p.calls++
	p.lastSQL = SQL
	p.lastParam = params
	return p.plan, p.err
}

func (p *plannerExecutor) ExplainCtx(ctx context.Context, SQL string, params ...interface{}) (*queryplan.Node, error) {
	p.lastCtx = ctx
	return p.Explain(SQL, params...)
}

func newPlannerExecutor(t *testing.T, plan *queryplan.Node, err error) *plannerExecutor {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	return &plannerExecutor{MockAdapter: mockgen.NewMockAdapter(ctrl), plan: plan, err: err}
}

func okScanner() adapters.Scanner {
	return &scanner.PrestScanner{Buff: bytes.NewBufferString(`[{"id":1}]`), IsQuery: true}
}

func seqScanPlan() *queryplan.Node {
	return &queryplan.Node{
		Operation: queryplan.OpSeqScan,
		NodeType:  "Seq Scan",
		Relation:  "orders",
		TotalCost: 1834.25,
		Rows:      98123,
	}
}

func indexScanPlan() *queryplan.Node {
	return &queryplan.Node{
		Operation: queryplan.OpIndexScan,
		NodeType:  "Index Scan",
		Relation:  "orders",
		TotalCost: 8.3,
		Rows:      1,
	}
}

func rejectSeqScan() Policies {
	return Policies{Default: Policy{RejectSeqScan: true}}
}

// Guarding requires an executor to guard.
func TestNewExecutorNilInner(t *testing.T) {
	t.Parallel()

	guarded, err := NewExecutor(nil, rejectSeqScan())
	require.ErrorIs(t, err, ErrNilExecutor)
	require.Nil(t, guarded)
}

// A policy set that enforces nothing returns the executor untouched, so no
// EXPLAIN round trip is paid for a guard that would never reject.
func TestNewExecutorZeroPoliciesReturnsInner(t *testing.T) {
	t.Parallel()

	inner := newPlannerExecutor(t, nil, nil)
	guarded, err := NewExecutor(inner, Policies{})
	require.NoError(t, err)
	require.Same(t, inner, guarded)
}

// Enforcing a policy on an executor that cannot explain statements is a
// configuration error: startup fails instead of serving traffic unprotected.
func TestNewExecutorWithoutPlannerFails(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	guarded, err := NewExecutor(mockgen.NewMockAdapter(ctrl), rejectSeqScan())
	require.ErrorIs(t, err, ErrPlannerUnsupported)
	require.Nil(t, guarded)
}

// An accepted plan runs the original statement with its parameters untouched.
func TestQueryCtxAccepted(t *testing.T) {
	t.Parallel()

	inner := newPlannerExecutor(t, indexScanPlan(), nil)
	sql := `SELECT * FROM orders WHERE id = $1`
	inner.EXPECT().QueryCtx(gomock.Any(), sql, 7).Return(okScanner())

	guarded, err := NewExecutor(inner, rejectSeqScan())
	require.NoError(t, err)

	ctx := context.Background()
	sc := guarded.QueryCtx(ctx, sql, 7)
	require.NoError(t, sc.Err())
	require.Equal(t, `[{"id":1}]`, string(sc.Bytes()))
	require.Equal(t, 1, inner.calls)
	require.Equal(t, sql, inner.lastSQL)
	require.Equal(t, []interface{}{7}, inner.lastParam)
	require.Equal(t, ctx, inner.lastCtx)
}

// A rejected plan never reaches the database; the scanner carries the rejection.
func TestQueryCtxRejected(t *testing.T) {
	t.Parallel()

	inner := newPlannerExecutor(t, seqScanPlan(), nil)

	guarded, err := NewExecutor(inner, rejectSeqScan())
	require.NoError(t, err)

	sc := guarded.QueryCtx(context.Background(), "SELECT * FROM orders")
	require.ErrorIs(t, sc.Err(), ErrRejected)
	rejection, ok := Rejection(sc.Err())
	require.True(t, ok)
	require.Equal(t, RuleSeqScan, rejection.Rule)
	require.Equal(t, "Sequential Scan detected on table 'orders'.", rejection.Reason)
	require.Nil(t, sc.Bytes())
}

// A statement the engine cannot plan is blocked, and the planning error is
// reported as itself rather than as a policy rejection.
func TestQueryCtxPlanningFailure(t *testing.T) {
	t.Parallel()

	planErr := errors.New(`relation "missing" does not exist`)
	inner := newPlannerExecutor(t, nil, planErr)

	guarded, err := NewExecutor(inner, rejectSeqScan())
	require.NoError(t, err)

	sc := guarded.QueryCtx(context.Background(), "SELECT * FROM missing")
	require.ErrorIs(t, sc.Err(), planErr)
	require.NotErrorIs(t, sc.Err(), ErrRejected)
	require.Contains(t, sc.Err().Error(), "query guard could not plan the query")
}

// A planner reporting neither plan nor error leaves nothing to evaluate, so the
// query is blocked instead of running unchecked.
func TestQueryCtxWithoutPlan(t *testing.T) {
	t.Parallel()

	inner := newPlannerExecutor(t, nil, nil)

	guarded, err := NewExecutor(inner, rejectSeqScan())
	require.NoError(t, err)

	sc := guarded.QueryCtx(context.Background(), "SELECT * FROM orders")
	require.ErrorIs(t, sc.Err(), ErrNoQueryPlan)
	require.NotErrorIs(t, sc.Err(), ErrRejected)
	require.Contains(t, sc.Err().Error(), "query guard could not plan the query")
}

// The database named in the request context selects its own policy, so a strict
// default can coexist with a permissive analytical database.
func TestQueryCtxUsesPerDatabasePolicy(t *testing.T) {
	t.Parallel()

	policies := Policies{
		Default:   Policy{RejectSeqScan: true},
		Databases: map[string]Policy{"analytics": {MaxCost: 100000}},
	}

	inner := newPlannerExecutor(t, seqScanPlan(), nil)
	inner.EXPECT().QueryCtx(gomock.Any(), "SELECT * FROM orders").Return(okScanner())

	guarded, err := NewExecutor(inner, policies)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), pctx.DBNameKey, "analytics")
	require.NoError(t, guarded.QueryCtx(ctx, "SELECT * FROM orders").Err())

	// The same statement on a database without an override hits the default policy.
	sc := guarded.QueryCtx(context.WithValue(context.Background(), pctx.DBNameKey, "billing"), "SELECT * FROM orders")
	require.ErrorIs(t, sc.Err(), ErrRejected)
}

// The non-context read path is guarded with the default policy.
func TestQuery(t *testing.T) {
	t.Parallel()

	t.Run("accepted", func(t *testing.T) {
		t.Parallel()
		inner := newPlannerExecutor(t, indexScanPlan(), nil)
		inner.EXPECT().Query("SELECT 1").Return(okScanner())

		guarded, err := NewExecutor(inner, rejectSeqScan())
		require.NoError(t, err)
		require.NoError(t, guarded.Query("SELECT 1").Err())
	})

	t.Run("rejected", func(t *testing.T) {
		t.Parallel()
		inner := newPlannerExecutor(t, seqScanPlan(), nil)

		guarded, err := NewExecutor(inner, rejectSeqScan())
		require.NoError(t, err)
		require.ErrorIs(t, guarded.Query("SELECT * FROM orders").Err(), ErrRejected)
	})
}

// Count queries scan the same tables as the rows they count, so they are guarded too.
func TestQueryCount(t *testing.T) {
	t.Parallel()

	t.Run("accepted", func(t *testing.T) {
		t.Parallel()
		inner := newPlannerExecutor(t, indexScanPlan(), nil)
		inner.EXPECT().QueryCount("SELECT COUNT(*) FROM orders").Return(okScanner())

		guarded, err := NewExecutor(inner, rejectSeqScan())
		require.NoError(t, err)
		require.NoError(t, guarded.QueryCount("SELECT COUNT(*) FROM orders").Err())
	})

	t.Run("rejected", func(t *testing.T) {
		t.Parallel()
		inner := newPlannerExecutor(t, seqScanPlan(), nil)

		guarded, err := NewExecutor(inner, rejectSeqScan())
		require.NoError(t, err)
		require.ErrorIs(t, guarded.QueryCount("SELECT COUNT(*) FROM orders").Err(), ErrRejected)
	})
}

func TestQueryCountCtx(t *testing.T) {
	t.Parallel()

	t.Run("accepted", func(t *testing.T) {
		t.Parallel()
		inner := newPlannerExecutor(t, indexScanPlan(), nil)
		inner.EXPECT().QueryCountCtx(gomock.Any(), "SELECT COUNT(*) FROM orders").Return(okScanner())

		guarded, err := NewExecutor(inner, rejectSeqScan())
		require.NoError(t, err)
		require.NoError(t, guarded.QueryCountCtx(context.Background(), "SELECT COUNT(*) FROM orders").Err())
	})

	t.Run("rejected", func(t *testing.T) {
		t.Parallel()
		inner := newPlannerExecutor(t, seqScanPlan(), nil)

		guarded, err := NewExecutor(inner, rejectSeqScan())
		require.NoError(t, err)
		require.ErrorIs(t, guarded.QueryCountCtx(context.Background(), "SELECT COUNT(*) FROM orders").Err(), ErrRejected)
	})
}

// Writes are not planned: the guard protects read load, and an INSERT plan says
// nothing useful about it.
func TestWritesAreNotGuarded(t *testing.T) {
	t.Parallel()

	inner := newPlannerExecutor(t, seqScanPlan(), nil)
	inner.EXPECT().InsertCtx(gomock.Any(), "INSERT INTO orders(id) VALUES($1)", 1).Return(okScanner())

	guarded, err := NewExecutor(inner, rejectSeqScan())
	require.NoError(t, err)

	require.NoError(t, guarded.InsertCtx(context.Background(), "INSERT INTO orders(id) VALUES($1)", 1).Err())
	require.Zero(t, inner.calls)
}

// GuardFor resolves a database alias to its guard, falling back to the default.
func TestGuardFor(t *testing.T) {
	t.Parallel()

	policies := Policies{
		Default:   Policy{MaxCost: 10},
		Databases: map[string]Policy{"Analytics": {MaxCost: 99}},
	}
	inner := newPlannerExecutor(t, nil, nil)
	guarded, err := NewExecutor(inner, policies)
	require.NoError(t, err)

	executor := guarded.(*Executor)
	require.Equal(t, float64(99), executor.GuardFor("analytics").Policy().MaxCost)
	require.Equal(t, float64(10), executor.GuardFor("billing").Policy().MaxCost)
	require.Equal(t, float64(10), executor.GuardFor("").Policy().MaxCost)
}

func TestDatabaseFromContext(t *testing.T) {
	t.Parallel()

	// A request without a database name falls back to the default policy, and a
	// missing context must not panic on the non-context read paths.
	var missing context.Context
	require.Empty(t, databaseFromContext(missing))
	require.Empty(t, databaseFromContext(context.Background()))
	require.Equal(t, "prest",
		databaseFromContext(context.WithValue(context.Background(), pctx.DBNameKey, "prest")))
}

func TestSummarize(t *testing.T) {
	t.Parallel()

	// Log summaries name the access path per relation, and omit the relation for
	// nodes that read none.
	plan := &queryplan.Node{
		NodeType: "Hash Join",
		Children: []queryplan.Node{{NodeType: "Seq Scan", Relation: "orders"}},
	}
	require.Equal(t, "Hash Join -> Seq Scan(orders)", summarize(plan))
}
