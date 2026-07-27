// Package queryplan models SQL execution plans and produces them from a
// database, independently of any storage adapter.
//
// Plans are obtained with EXPLAIN and never with EXPLAIN ANALYZE, so a statement
// is planned but not executed. Engine-native plan formats are normalized into
// Node, letting callers reason about access paths without knowing the dialect.
//
// A Planner is built once per adapter and injected into it, so the SQL adapters
// keep owning connections while this package owns plan parsing.
package queryplan

// Operation is the engine-agnostic classification of an execution plan node.
//
// Adapters map their native node names onto these values so callers can be
// written once and evaluated against any SQL engine.
type Operation string

const (
	// OpSeqScan is a full scan of a relation without index support.
	OpSeqScan Operation = "seq_scan"
	// OpParallelSeqScan is a sequential scan executed by parallel workers.
	OpParallelSeqScan Operation = "parallel_seq_scan"
	// OpIndexScan is any scan served by an index.
	OpIndexScan Operation = "index_scan"
	// OpJoin combines rows from two inputs (nested loop, hash, merge).
	OpJoin Operation = "join"
	// OpOther covers nodes no caller reasons about (sort, limit, ...).
	OpOther Operation = "other"
)

// Node is one node of a normalized execution plan tree.
//
// Cost and row figures are planner estimates, not measurements: plans are
// obtained without executing the statement.
type Node struct {
	// Operation is the normalized classification of this node.
	Operation Operation
	// NodeType is the engine-native node name, kept for diagnostics and error
	// messages (e.g. "Seq Scan", "Bitmap Heap Scan").
	NodeType string
	// Relation is the table this node reads, when the node reads one.
	Relation string
	// TotalCost is the estimated cost to return every row of this node.
	TotalCost float64
	// Rows is the estimated number of rows this node emits.
	Rows float64
	// Children are the input nodes feeding this one.
	Children []Node
}

// Walk calls fn for n and every descendant, depth first. It stops early and
// returns false when fn returns false.
func (n *Node) Walk(fn func(*Node) bool) bool {
	if n == nil {
		return true
	}
	if !fn(n) {
		return false
	}
	for i := range n.Children {
		if !n.Children[i].Walk(fn) {
			return false
		}
	}
	return true
}
