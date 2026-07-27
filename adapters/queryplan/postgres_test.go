package queryplan

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// seqScanPlanJSON is a postgres plan for an unindexed filter over one table.
const seqScanPlanJSON = `[{"Plan":{"Node Type":"Seq Scan","Parallel Aware":false,` +
	`"Relation Name":"orders","Total Cost":1834.25,"Plan Rows":98123,"Plan Width":40}}]`

// joinPlanJSON nests an index scan and a parallel sequential scan under a hash join.
const joinPlanJSON = `[{"Plan":{"Node Type":"Hash Join","Parallel Aware":false,` +
	`"Total Cost":5000.5,"Plan Rows":1200,"Plans":[` +
	`{"Node Type":"Index Scan","Parallel Aware":false,"Relation Name":"customers",` +
	`"Total Cost":8.3,"Plan Rows":1},` +
	`{"Node Type":"Seq Scan","Parallel Aware":true,"Relation Name":"orders",` +
	`"Total Cost":900.1,"Plan Rows":50000}]}}]`

// stubExplainer stands in for the adapter that owns the connection and the
// prepared statement lifecycle.
type stubExplainer struct {
	raw       []byte
	err       error
	lastSQL   string
	lastCtx   context.Context
	lastParam []interface{}
}

func (s *stubExplainer) ExplainRow(ctx context.Context, SQL string, params ...interface{}) ([]byte, error) {
	s.lastCtx = ctx
	s.lastSQL = SQL
	s.lastParam = params
	return s.raw, s.err
}

// The planner prefixes the statement with EXPLAIN (FORMAT JSON), forwards the
// caller's parameters untouched, and returns the normalized tree.
func TestPostgresExplainCtx(t *testing.T) {
	t.Parallel()

	explainer := &stubExplainer{raw: []byte(seqScanPlanJSON)}
	ctx := context.Background()

	plan, err := NewPostgres(explainer).ExplainCtx(ctx,
		"SELECT * FROM orders WHERE status = $1", "open")

	require.NoError(t, err)
	require.Equal(t, OpSeqScan, plan.Operation)
	require.Equal(t, "Seq Scan", plan.NodeType)
	require.Equal(t, "orders", plan.Relation)
	require.InDelta(t, 1834.25, plan.TotalCost, 0.001)
	require.InDelta(t, 98123, plan.Rows, 0.001)
	require.Equal(t, "EXPLAIN (FORMAT JSON) SELECT * FROM orders WHERE status = $1", explainer.lastSQL)
	require.Equal(t, []interface{}{"open"}, explainer.lastParam)
	require.Equal(t, ctx, explainer.lastCtx)
}

// The non-context entry point plans through the same path.
func TestPostgresExplain(t *testing.T) {
	t.Parallel()

	explainer := &stubExplainer{raw: []byte(seqScanPlanJSON)}

	plan, err := NewPostgres(explainer).Explain("SELECT 1")

	require.NoError(t, err)
	require.Equal(t, "orders", plan.Relation)
	require.Equal(t, "EXPLAIN (FORMAT JSON) SELECT 1", explainer.lastSQL)
}

// Child nodes keep their own classification so callers can inspect the subtree.
func TestPostgresExplainNestedNodes(t *testing.T) {
	t.Parallel()

	plan, err := NewPostgres(&stubExplainer{raw: []byte(joinPlanJSON)}).
		ExplainCtx(context.Background(), "SELECT 1")

	require.NoError(t, err)
	require.Equal(t, OpJoin, plan.Operation)
	require.Len(t, plan.Children, 2)
	require.Equal(t, OpIndexScan, plan.Children[0].Operation)
	require.Equal(t, OpParallelSeqScan, plan.Children[1].Operation)
	require.Equal(t, "orders", plan.Children[1].Relation)
}

// A database failure surfaces as itself, not as a partial plan.
func TestPostgresExplainError(t *testing.T) {
	t.Parallel()

	explainErr := errors.New(`relation "missing" does not exist`)

	plan, err := NewPostgres(&stubExplainer{err: explainErr}).
		ExplainCtx(context.Background(), "SELECT * FROM missing")

	require.ErrorIs(t, err, explainErr)
	require.Nil(t, plan)
}

// A planner without a way to reach the database cannot plan anything.
func TestPostgresExplainWithoutExplainer(t *testing.T) {
	t.Parallel()

	plan, err := NewPostgres(nil).ExplainCtx(context.Background(), "SELECT 1")
	require.ErrorIs(t, err, ErrNoExplainer)
	require.Nil(t, plan)

	var nilPlanner *Postgres
	plan, err = nilPlanner.Explain("SELECT 1")
	require.ErrorIs(t, err, ErrNoExplainer)
	require.Nil(t, plan)
}

func TestParsePostgresPlan(t *testing.T) {
	t.Parallel()

	// An empty payload is a protocol violation, not a plan with no nodes.
	t.Run("empty payload", func(t *testing.T) {
		t.Parallel()
		plan, err := parsePostgresPlan(nil)
		require.ErrorIs(t, err, ErrEmptyPlan)
		require.Nil(t, plan)
	})

	// An empty JSON array carries no plan either.
	t.Run("empty array", func(t *testing.T) {
		t.Parallel()
		plan, err := parsePostgresPlan([]byte(`[]`))
		require.ErrorIs(t, err, ErrEmptyPlan)
		require.Nil(t, plan)
	})

	// Malformed JSON is reported as a decode failure.
	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		plan, err := parsePostgresPlan([]byte(`{`))
		require.Error(t, err)
		require.Nil(t, plan)
	})

	// A leaf node carries no children slice at all.
	t.Run("leaf has no children", func(t *testing.T) {
		t.Parallel()
		plan, err := parsePostgresPlan([]byte(seqScanPlanJSON))
		require.NoError(t, err)
		require.Nil(t, plan.Children)
	})
}

func TestClassifyNodeType(t *testing.T) {
	t.Parallel()

	// Each postgres node name maps to exactly one normalized operation; unknown
	// nodes fall back to "other" so callers ignore them.
	cases := map[string]struct {
		nodeType string
		parallel bool
		want     Operation
	}{
		"seq scan":          {"Seq Scan", false, OpSeqScan},
		"parallel seq scan": {"Seq Scan", true, OpParallelSeqScan},
		"index scan":        {"Index Scan", false, OpIndexScan},
		"index only scan":   {"Index Only Scan", false, OpIndexScan},
		"bitmap index scan": {"Bitmap Index Scan", false, OpIndexScan},
		"bitmap heap scan":  {"Bitmap Heap Scan", false, OpIndexScan},
		"nested loop":       {"Nested Loop", false, OpJoin},
		"hash join":         {"Hash Join", false, OpJoin},
		"merge join":        {"Merge Join", false, OpJoin},
		"sort":              {"Sort", false, OpOther},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, classifyNodeType(tc.nodeType, tc.parallel))
		})
	}
}

// The postgres planner must satisfy the port it is injected as.
func TestPostgresImplementsPlanner(t *testing.T) {
	t.Parallel()
	require.Implements(t, (*Planner)(nil), NewPostgres(nil))
}
