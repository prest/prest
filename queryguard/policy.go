// Package queryguard rejects queries whose execution plan violates a
// configurable performance policy.
//
// It exists for deployments that expose pREST to third parties (vendors, BI
// tools, AI agents) where protecting the database matters more than query
// flexibility: an unindexed filter over a large table is refused before
// PostgreSQL ever runs it.
//
// The guard decorates an adapters.QueryExecutor. Read statements are first
// planned through the queryplan.Planner port (EXPLAIN, never EXPLAIN
// ANALYZE) and the resulting plan is matched against the policy of the database
// named in the request context. Writes are not planned.
//
// The statement handed to the guard is the one the handler generated, before the
// adapter wraps it in its JSON aggregation. The aggregate adds a node on top of
// the plan but changes no scan, join or row estimate, which is what the rules
// reason about.
package queryguard

import "strings"

// Policy describes the execution-plan rules a read query must satisfy.
//
// Numeric limits are disabled when zero or negative, so the zero Policy accepts
// every plan.
type Policy struct {
	// RejectSeqScan refuses plans containing a sequential scan.
	RejectSeqScan bool
	// RejectParallelSeqScan refuses plans containing a parallel sequential scan.
	RejectParallelSeqScan bool
	// MaxCost is the highest estimated total cost the root node may report.
	MaxCost float64
	// MaxRows is the highest estimated row count the root node may report.
	MaxRows float64
	// RequireIndexUsage refuses plans that read no relation through an index.
	RequireIndexUsage bool
	// MaxJoins is the highest number of join nodes a plan may contain.
	MaxJoins int
	// AllowTables lists relations exempt from the scan rules (RejectSeqScan,
	// RejectParallelSeqScan and RequireIndexUsage). Use it for small lookup
	// tables where a full scan is cheaper than an index.
	AllowTables []string
}

// IsZero reports whether the policy enforces nothing, in which case wrapping an
// adapter only adds an EXPLAIN round trip for no benefit.
func (p Policy) IsZero() bool {
	return !p.RejectSeqScan &&
		!p.RejectParallelSeqScan &&
		!p.RequireIndexUsage &&
		p.MaxCost <= 0 &&
		p.MaxRows <= 0 &&
		p.MaxJoins <= 0
}

// allowSet indexes AllowTables for lookup. Relation names are compared
// case-insensitively because postgres folds unquoted identifiers to lower case.
func (p Policy) allowSet() map[string]struct{} {
	if len(p.AllowTables) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(p.AllowTables))
	for _, table := range p.AllowTables {
		table = strings.ToLower(strings.TrimSpace(table))
		if table == "" {
			continue
		}
		set[table] = struct{}{}
	}
	return set
}
