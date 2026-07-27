package postgres_test

import (
	"context"
	"testing"

	"github.com/prest/prest/v2/adapters/queryplan"
	pctx "github.com/prest/prest/v2/context"

	"github.com/stretchr/testify/require"
)

// planner returns the test adapter as the optional query planner port.
func planner(t *testing.T) queryplan.Planner {
	t.Helper()
	p, ok := testAdapter(t).(queryplan.Planner)
	require.True(t, ok, "postgres adapter must implement queryplan.Planner")
	return p
}

// explainCtx returns a context naming the integration database, as the CRUD
// handlers do for every request.
func explainCtx() context.Context {
	return context.WithValue(context.Background(), pctx.DBNameKey, "prest-test") //nolint:staticcheck
}

// An unqualified read of a table without a usable index is planned as a
// sequential scan, with the estimates the policy rules compare against.
func TestExplainCtx_SeqScanAgainstRealDatabase(t *testing.T) {
	plan, err := planner(t).ExplainCtx(explainCtx(), `SELECT * FROM "public"."test"`)

	require.NoError(t, err)
	require.Equal(t, queryplan.OpSeqScan, plan.Operation)
	require.Equal(t, "Seq Scan", plan.NodeType)
	require.Equal(t, "test", plan.Relation)
	require.Greater(t, plan.TotalCost, float64(0))
	require.Greater(t, plan.Rows, float64(0))
}

// Parameter placeholders are planned, not executed: the statement is prepared
// with the caller's values so the planner sees the real query shape.
func TestExplainCtx_WithParameters(t *testing.T) {
	plan, err := planner(t).ExplainCtx(explainCtx(), `SELECT * FROM "public"."test" WHERE "name" = $1`, "prest")

	require.NoError(t, err)
	require.NotEmpty(t, plan.NodeType)
	require.Equal(t, "test", plan.Relation)
}

// A join plan keeps its children, so rules that walk the tree see every relation.
func TestExplainCtx_JoinKeepsChildren(t *testing.T) {
	plan, err := planner(t).ExplainCtx(explainCtx(),
		`SELECT t.id FROM "public"."test" t JOIN "public"."test2" t2 ON t.name = t2.name`)

	require.NoError(t, err)
	relations := map[string]bool{}
	plan.Walk(func(node *queryplan.Node) bool {
		if node.Relation != "" {
			relations[node.Relation] = true
		}
		return true
	})
	require.True(t, relations["test"], "plan should reference table test")
	require.True(t, relations["test2"], "plan should reference table test2")
}

// A statement postgres cannot plan reports the database error instead of an
// empty plan, so the guard can block it without guessing.
func TestExplainCtx_UnknownRelation(t *testing.T) {
	plan, err := planner(t).ExplainCtx(explainCtx(), `SELECT * FROM "public"."does_not_exist_9fa1"`)

	require.Error(t, err)
	require.Nil(t, plan)
}
