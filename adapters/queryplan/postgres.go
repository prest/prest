package queryplan

import (
	"context"
	"encoding/json"
	"fmt"
)

// explainPrefix asks postgres for a machine readable plan. FORMAT JSON keeps the
// whole plan in a single row/column, and no ANALYZE is used so the statement is
// planned but never executed.
const explainPrefix = "EXPLAIN (FORMAT JSON) "

// Postgres plans statements through the PostgreSQL EXPLAIN command. It serves
// engines whose EXPLAIN accepts FORMAT JSON and reports the PostgreSQL plan
// shape — PostgreSQL itself and forks built on its query layer, such as
// TimescaleDB, Aurora PostgreSQL and YugabyteDB. Engines that are only
// wire-compatible, and whose EXPLAIN has its own syntax and output, need their
// own Planner.
type Postgres struct {
	explainer Explainer
}

// NewPostgres builds a planner that reaches the database through explainer.
func NewPostgres(explainer Explainer) *Postgres {
	return &Postgres{explainer: explainer}
}

// Explain returns the execution plan for SQL on the adapter's default connection.
func (p *Postgres) Explain(SQL string, params ...interface{}) (*Node, error) {
	return p.ExplainCtx(context.Background(), SQL, params...)
}

// ExplainCtx returns the execution plan for SQL on the connection selected for
// ctx. The statement is prepared with the caller's parameters so the planner sees
// the same query shape that execution would use.
func (p *Postgres) ExplainCtx(ctx context.Context, SQL string, params ...interface{}) (*Node, error) {
	if p == nil || p.explainer == nil {
		return nil, ErrNoExplainer
	}
	raw, err := p.explainer.ExplainRow(ctx, explainPrefix+SQL, params...)
	if err != nil {
		return nil, err
	}
	return parsePostgresPlan(raw)
}

// explainNode mirrors the postgres EXPLAIN (FORMAT JSON) node shape.
type explainNode struct {
	NodeType      string        `json:"Node Type"`
	ParallelAware bool          `json:"Parallel Aware"`
	RelationName  string        `json:"Relation Name"`
	TotalCost     float64       `json:"Total Cost"`
	PlanRows      float64       `json:"Plan Rows"`
	Plans         []explainNode `json:"Plans"`
}

// explainResult is one entry of the top level EXPLAIN array.
type explainResult struct {
	Plan explainNode `json:"Plan"`
}

// parsePostgresPlan converts a postgres EXPLAIN (FORMAT JSON) payload into the
// engine-agnostic plan tree.
func parsePostgresPlan(raw []byte) (*Node, error) {
	if len(raw) == 0 {
		return nil, ErrEmptyPlan
	}
	var results []explainResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, fmt.Errorf("decode query plan: %w", err)
	}
	if len(results) == 0 {
		return nil, ErrEmptyPlan
	}
	node := normalizeNode(results[0].Plan)
	return &node, nil
}

// normalizeNode maps one postgres node (and its subtree) onto Node.
func normalizeNode(n explainNode) Node {
	node := Node{
		Operation: classifyNodeType(n.NodeType, n.ParallelAware),
		NodeType:  n.NodeType,
		Relation:  n.RelationName,
		TotalCost: n.TotalCost,
		Rows:      n.PlanRows,
	}
	if len(n.Plans) == 0 {
		return node
	}

	node.Children = make([]Node, 0, len(n.Plans))
	for _, child := range n.Plans {
		node.Children = append(node.Children, normalizeNode(child))
	}
	return node
}

// classifyNodeType maps a postgres node name onto a normalized operation.
// Bitmap scans count as index scans: they are driven by an index even though the
// heap access happens in a separate node.
func classifyNodeType(nodeType string, parallelAware bool) Operation {
	switch nodeType {
	case "Seq Scan":
		if parallelAware {
			return OpParallelSeqScan
		}
		return OpSeqScan
	case "Index Scan", "Index Only Scan", "Bitmap Index Scan", "Bitmap Heap Scan":
		return OpIndexScan
	case "Nested Loop", "Hash Join", "Merge Join":
		return OpJoin
	default:
		return OpOther
	}
}
