package queryguard

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/adapters/queryplan"
)

// ErrRejected marks every error produced by a policy violation. Callers use
// errors.Is to distinguish a rejected query from a database failure.
var ErrRejected = errors.New("query rejected by Query Guard")

// Rule identifiers, reported on rejections so operators can tell which setting
// refused the query and can aggregate rejections per rule.
const (
	RuleSeqScan           = "reject_seq_scan"
	RuleParallelSeqScan   = "reject_parallel_seq_scan"
	RuleMaxCost           = "max_cost"
	RuleMaxRows           = "max_rows"
	RuleMaxJoins          = "max_joins"
	RuleRequireIndexUsage = "require_index_usage"
)

// RejectionError reports the rule that refused a query and a human readable
// reason suitable for an API response.
type RejectionError struct {
	Rule   string
	Reason string
}

// Error implements error.
func (e *RejectionError) Error() string {
	return fmt.Sprintf("%s: %s", ErrRejected.Error(), e.Reason)
}

// Unwrap makes errors.Is(err, ErrRejected) report true for any rejection.
func (e *RejectionError) Unwrap() error { return ErrRejected }

// Rejection extracts the rejection details from err, when err was produced by a
// policy violation.
func Rejection(err error) (*RejectionError, bool) {
	var rejection *RejectionError
	if errors.As(err, &rejection) {
		return rejection, true
	}
	return nil, false
}

// Guard evaluates execution plans against a policy.
type Guard struct {
	policy Policy
	allow  map[string]struct{}
}

// New builds a Guard for policy.
func New(policy Policy) *Guard {
	return &Guard{policy: policy, allow: policy.allowSet()}
}

// Policy returns the policy this guard enforces.
func (g *Guard) Policy() Policy { return g.policy }

// Check reports the first policy violation found in plan, or nil when the query
// may run. A nil plan is accepted: the caller decides how to treat an engine
// that produced no plan.
func (g *Guard) Check(plan *queryplan.Node) error {
	if g == nil || plan == nil {
		return nil
	}
	if err := g.checkScans(plan); err != nil {
		return err
	}
	if err := g.checkEstimates(plan); err != nil {
		return err
	}
	if err := g.checkJoins(plan); err != nil {
		return err
	}
	return g.checkIndexUsage(plan)
}

// checkScans refuses full table scans, skipping relations the policy allows.
func (g *Guard) checkScans(plan *queryplan.Node) error {
	if !g.policy.RejectSeqScan && !g.policy.RejectParallelSeqScan {
		return nil
	}
	var violation *RejectionError
	plan.Walk(func(node *queryplan.Node) bool {
		if g.allowed(node.Relation) {
			return true
		}
		switch node.Operation {
		case queryplan.OpSeqScan:
			if g.policy.RejectSeqScan {
				violation = &RejectionError{
					Rule:   RuleSeqScan,
					Reason: fmt.Sprintf("Sequential Scan detected%s.", onTable(node.Relation)),
				}
				return false
			}
		case queryplan.OpParallelSeqScan:
			if g.policy.RejectParallelSeqScan {
				violation = &RejectionError{
					Rule:   RuleParallelSeqScan,
					Reason: fmt.Sprintf("Parallel Sequential Scan detected%s.", onTable(node.Relation)),
				}
				return false
			}
			// A parallel scan is still a sequential scan: honour the broader rule
			// when only reject_seq_scan is enabled.
			if g.policy.RejectSeqScan {
				violation = &RejectionError{
					Rule:   RuleSeqScan,
					Reason: fmt.Sprintf("Sequential Scan detected%s.", onTable(node.Relation)),
				}
				return false
			}
		}
		return true
	})
	if violation != nil {
		return violation
	}
	return nil
}

// checkEstimates refuses plans whose root estimates exceed the configured caps.
func (g *Guard) checkEstimates(plan *queryplan.Node) error {
	if g.policy.MaxCost > 0 && plan.TotalCost > g.policy.MaxCost {
		return &RejectionError{
			Rule: RuleMaxCost,
			Reason: fmt.Sprintf("Estimated cost %s exceeds the maximum allowed cost of %s.",
				formatFloat(plan.TotalCost), formatFloat(g.policy.MaxCost)),
		}
	}
	if g.policy.MaxRows > 0 && plan.Rows > g.policy.MaxRows {
		return &RejectionError{
			Rule: RuleMaxRows,
			Reason: fmt.Sprintf("Estimated row count %s exceeds the maximum allowed of %s.",
				formatFloat(plan.Rows), formatFloat(g.policy.MaxRows)),
		}
	}
	return nil
}

// checkJoins refuses plans with more join nodes than the policy allows.
func (g *Guard) checkJoins(plan *queryplan.Node) error {
	if g.policy.MaxJoins <= 0 {
		return nil
	}
	joins := 0
	plan.Walk(func(node *queryplan.Node) bool {
		if node.Operation == queryplan.OpJoin {
			joins++
		}
		return true
	})
	if joins > g.policy.MaxJoins {
		return &RejectionError{
			Rule: RuleMaxJoins,
			Reason: fmt.Sprintf("Query performs %d joins, the maximum allowed is %d.",
				joins, g.policy.MaxJoins),
		}
	}
	return nil
}

// checkIndexUsage refuses plans that read no relation through an index. Queries
// touching only allowed relations pass: those tables opted out of index rules.
func (g *Guard) checkIndexUsage(plan *queryplan.Node) error {
	if !g.policy.RequireIndexUsage {
		return nil
	}
	usesIndex := false
	guarded := false
	plan.Walk(func(node *queryplan.Node) bool {
		if node.Operation == queryplan.OpIndexScan {
			usesIndex = true
			return false
		}
		if isScan(node.Operation) && !g.allowed(node.Relation) {
			guarded = true
		}
		return true
	})
	if usesIndex || !guarded {
		return nil
	}
	return &RejectionError{
		Rule:   RuleRequireIndexUsage,
		Reason: "Query plan uses no index; at least one indexed predicate is required.",
	}
}

// allowed reports whether relation is exempt from the scan rules.
func (g *Guard) allowed(relation string) bool {
	if relation == "" || len(g.allow) == 0 {
		return false
	}
	_, ok := g.allow[strings.ToLower(relation)]
	return ok
}

// isScan reports whether op reads a relation directly.
func isScan(op queryplan.Operation) bool {
	return op == queryplan.OpSeqScan || op == queryplan.OpParallelSeqScan
}

// onTable renders the relation suffix of a rejection reason, tolerating plan
// nodes that report no relation (subqueries, CTEs, function scans).
func onTable(relation string) string {
	if relation == "" {
		return " in the query plan"
	}
	return fmt.Sprintf(" on table '%s'", relation)
}

// formatFloat renders planner estimates without exponent notation.
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}
