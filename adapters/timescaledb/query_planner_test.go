package timescaledb

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/adapters/queryplan"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// plannerAdapter is a mock adapter that can also explain statements, like the
// postgres adapter TimescaleDB wraps in production.
type plannerAdapter struct {
	*mockgen.MockAdapter

	plan *queryplan.Node
	ctx  context.Context
	sql  string
}

func (p *plannerAdapter) Explain(SQL string, _ ...interface{}) (*queryplan.Node, error) {
	p.sql = SQL
	return p.plan, nil
}

func (p *plannerAdapter) ExplainCtx(ctx context.Context, SQL string, _ ...interface{}) (*queryplan.Node, error) {
	p.ctx = ctx
	p.sql = SQL
	return p.plan, nil
}

// Planning is delegated to the wrapped adapter: TimescaleDB shares the postgres
// EXPLAIN output and adds nothing of its own.
func TestExplainDelegates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plan := &queryplan.Node{NodeType: "Seq Scan", Relation: "metrics"}
	inner := &plannerAdapter{MockAdapter: mockgen.NewMockAdapter(ctrl), plan: plan}
	adapter := &Adapter{Adapter: inner}

	got, err := adapter.Explain("SELECT 1")
	require.NoError(t, err)
	require.Same(t, plan, got)
	require.Equal(t, "SELECT 1", inner.sql)

	ctx := context.Background()
	got, err = adapter.ExplainCtx(ctx, "SELECT 2")
	require.NoError(t, err)
	require.Same(t, plan, got)
	require.Equal(t, "SELECT 2", inner.sql)
	require.Equal(t, ctx, inner.ctx)
}

// An adapter that cannot explain statements reports it instead of pretending a
// plan exists, so Query Guard fails closed.
func TestExplainWithoutPlanner(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := &Adapter{Adapter: mockgen.NewMockAdapter(ctrl)}

	plan, err := adapter.Explain("SELECT 1")
	require.ErrorIs(t, err, ErrNoQueryPlanner)
	require.Nil(t, plan)

	plan, err = adapter.ExplainCtx(context.Background(), "SELECT 1")
	require.ErrorIs(t, err, ErrNoQueryPlanner)
	require.Nil(t, plan)
}

// The TimescaleDB adapter must satisfy the optional planner port, so the guard
// can be enabled on TimescaleDB databases.
func TestTimescaleImplementsQueryPlanner(t *testing.T) {
	t.Parallel()
	require.Implements(t, (*queryplan.Planner)(nil), &Adapter{})
}
