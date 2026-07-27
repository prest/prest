package queryguard

import (
	"errors"
	"testing"

	"github.com/prest/prest/v2/adapters/queryplan"

	"github.com/stretchr/testify/require"
)

func seqScan(relation string, cost, rows float64) *queryplan.Node {
	return &queryplan.Node{
		Operation: queryplan.OpSeqScan,
		NodeType:  "Seq Scan",
		Relation:  relation,
		TotalCost: cost,
		Rows:      rows,
	}
}

func indexScan(relation string, cost, rows float64) queryplan.Node {
	return queryplan.Node{
		Operation: queryplan.OpIndexScan,
		NodeType:  "Index Scan",
		Relation:  relation,
		TotalCost: cost,
		Rows:      rows,
	}
}

// An empty policy accepts everything, including the worst possible plan.
func TestCheckEmptyPolicyAccepts(t *testing.T) {
	t.Parallel()
	require.NoError(t, New(Policy{}).Check(seqScan("orders", 999999, 999999)))
}

// A nil guard or a nil plan must not panic: an engine that returned no plan is
// the caller's problem to report, not a policy violation.
func TestCheckNilInputs(t *testing.T) {
	t.Parallel()

	var guard *Guard
	require.NoError(t, guard.Check(seqScan("orders", 1, 1)))
	require.NoError(t, New(Policy{RejectSeqScan: true}).Check(nil))
}

// reject_seq_scan refuses a full scan and names the offending table.
func TestCheckRejectSeqScan(t *testing.T) {
	t.Parallel()

	err := New(Policy{RejectSeqScan: true}).Check(seqScan("orders", 1834.25, 98123))

	require.ErrorIs(t, err, ErrRejected)
	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleSeqScan, rejection.Rule)
	require.Equal(t, "Sequential Scan detected on table 'orders'.", rejection.Reason)
	require.Contains(t, err.Error(), "query rejected by Query Guard")
}

// Nodes without a relation (subquery, CTE, function scan) still get a readable
// rejection reason.
func TestCheckRejectSeqScanWithoutRelation(t *testing.T) {
	t.Parallel()

	err := New(Policy{RejectSeqScan: true}).Check(seqScan("", 10, 10))

	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, "Sequential Scan detected in the query plan.", rejection.Reason)
}

// A scan buried under a join is found: the whole tree is inspected, not the root.
func TestCheckRejectSeqScanInSubtree(t *testing.T) {
	t.Parallel()

	plan := &queryplan.Node{
		Operation: queryplan.OpJoin,
		NodeType:  "Hash Join",
		TotalCost: 200,
		Children:  []queryplan.Node{indexScan("customers", 8, 1), *seqScan("orders", 190, 5000)},
	}

	err := New(Policy{RejectSeqScan: true}).Check(plan)

	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleSeqScan, rejection.Rule)
	require.Contains(t, rejection.Reason, "orders")
}

// Tables in allow_tables opt out of the scan rules, so lookup tables stay usable.
func TestCheckAllowTablesExemptFromSeqScan(t *testing.T) {
	t.Parallel()

	policy := Policy{RejectSeqScan: true, AllowTables: []string{"Countries"}}
	require.NoError(t, New(policy).Check(seqScan("countries", 5, 250)))
}

// reject_parallel_seq_scan targets parallel scans specifically.
func TestCheckRejectParallelSeqScan(t *testing.T) {
	t.Parallel()

	plan := &queryplan.Node{
		Operation: queryplan.OpParallelSeqScan,
		NodeType:  "Seq Scan",
		Relation:  "orders",
		TotalCost: 900,
	}

	err := New(Policy{RejectParallelSeqScan: true}).Check(plan)

	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleParallelSeqScan, rejection.Rule)
	require.Equal(t, "Parallel Sequential Scan detected on table 'orders'.", rejection.Reason)
}

// A parallel scan is still a sequential scan, so reject_seq_scan alone catches it.
func TestCheckRejectSeqScanCatchesParallelScan(t *testing.T) {
	t.Parallel()

	plan := &queryplan.Node{
		Operation: queryplan.OpParallelSeqScan,
		NodeType:  "Seq Scan",
		Relation:  "orders",
	}

	err := New(Policy{RejectSeqScan: true}).Check(plan)

	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleSeqScan, rejection.Rule)
}

// Enabling only reject_parallel_seq_scan leaves ordinary scans allowed.
func TestCheckParallelRuleIgnoresSerialScan(t *testing.T) {
	t.Parallel()
	require.NoError(t, New(Policy{RejectParallelSeqScan: true}).Check(seqScan("orders", 10, 10)))
}

// max_cost compares the root estimate against the configured ceiling.
func TestCheckMaxCost(t *testing.T) {
	t.Parallel()

	guard := New(Policy{MaxCost: 50000})

	require.NoError(t, guard.Check(&queryplan.Node{TotalCost: 50000}))

	err := guard.Check(&queryplan.Node{TotalCost: 78000.5})
	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleMaxCost, rejection.Rule)
	require.Equal(t, "Estimated cost 78000.50 exceeds the maximum allowed cost of 50000.00.", rejection.Reason)
}

// max_rows compares the root row estimate against the configured ceiling.
func TestCheckMaxRows(t *testing.T) {
	t.Parallel()

	guard := New(Policy{MaxRows: 100000})

	require.NoError(t, guard.Check(&queryplan.Node{Rows: 100000}))

	err := guard.Check(&queryplan.Node{Rows: 250000})
	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleMaxRows, rejection.Rule)
	require.Contains(t, rejection.Reason, "250000.00")
}

// max_joins counts join nodes anywhere in the tree.
func TestCheckMaxJoins(t *testing.T) {
	t.Parallel()

	plan := &queryplan.Node{
		Operation: queryplan.OpJoin,
		NodeType:  "Hash Join",
		Children: []queryplan.Node{
			{Operation: queryplan.OpJoin, NodeType: "Nested Loop"},
			indexScan("customers", 8, 1),
		},
	}

	require.NoError(t, New(Policy{MaxJoins: 2}).Check(plan))

	err := New(Policy{MaxJoins: 1}).Check(plan)
	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleMaxJoins, rejection.Rule)
	require.Equal(t, "Query performs 2 joins, the maximum allowed is 1.", rejection.Reason)
}

// require_index_usage accepts any plan that reads through an index.
func TestCheckRequireIndexUsageAccepted(t *testing.T) {
	t.Parallel()

	plan := &queryplan.Node{
		Operation: queryplan.OpJoin,
		NodeType:  "Hash Join",
		Children:  []queryplan.Node{indexScan("orders", 8, 1), *seqScan("customers", 5, 10)},
	}

	require.NoError(t, New(Policy{RequireIndexUsage: true}).Check(plan))
}

// A plan with no index access at all is refused.
func TestCheckRequireIndexUsageRejected(t *testing.T) {
	t.Parallel()

	err := New(Policy{RequireIndexUsage: true}).Check(seqScan("orders", 1000, 50000))

	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleRequireIndexUsage, rejection.Rule)
	require.Equal(t, "Query plan uses no index; at least one indexed predicate is required.", rejection.Reason)
}

// When every scanned relation is allow-listed there is nothing left to protect,
// so the index requirement does not apply.
func TestCheckRequireIndexUsageSkipsAllowedTables(t *testing.T) {
	t.Parallel()

	policy := Policy{RequireIndexUsage: true, AllowTables: []string{"countries"}}
	require.NoError(t, New(policy).Check(seqScan("countries", 5, 250)))
}

// A plan that reads no relation (e.g. SELECT 1) has nothing to index.
func TestCheckRequireIndexUsageWithoutScans(t *testing.T) {
	t.Parallel()

	plan := &queryplan.Node{Operation: queryplan.OpOther, NodeType: "Result"}
	require.NoError(t, New(Policy{RequireIndexUsage: true}).Check(plan))
}

// Scan rules are reported before cost rules so the message points at the cause.
func TestCheckScanRuleTakesPrecedence(t *testing.T) {
	t.Parallel()

	policy := Policy{RejectSeqScan: true, MaxCost: 1}
	err := New(policy).Check(seqScan("orders", 5000, 10))

	rejection, ok := Rejection(err)
	require.True(t, ok)
	require.Equal(t, RuleSeqScan, rejection.Rule)
}

// Policy returns the configuration the guard was built with.
func TestGuardPolicy(t *testing.T) {
	t.Parallel()

	policy := Policy{MaxCost: 10}
	require.Equal(t, policy, New(policy).Policy())
}

// Rejection reports false for unrelated errors, so callers cannot mistake a
// database failure for a policy violation.
func TestRejectionOnUnrelatedError(t *testing.T) {
	t.Parallel()

	rejection, ok := Rejection(errors.New("connection refused"))
	require.False(t, ok)
	require.Nil(t, rejection)
}
