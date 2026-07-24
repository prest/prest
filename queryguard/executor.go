package queryguard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/queryplan"
	"github.com/prest/prest/v2/adapters/scanner"
	pctx "github.com/prest/prest/v2/context"
)

// ErrNilExecutor is returned when there is no executor to guard.
var ErrNilExecutor = errors.New("query guard requires a query executor")

// ErrPlannerUnsupported is returned when the wrapped executor cannot produce
// execution plans, which makes the policy unenforceable.
var ErrPlannerUnsupported = errors.New("executor does not implement queryplan.Planner; query guard cannot be enabled")

// ErrNoQueryPlan is returned when the planner reports neither a plan nor an
// error. There is nothing to evaluate, so the query cannot be cleared.
var ErrNoQueryPlan = errors.New("planner returned no execution plan")

// Executor decorates an adapters.QueryExecutor: read statements are planned
// through queryplan.Planner (EXPLAIN, never EXPLAIN ANALYZE) and refused
// when the plan violates the policy of the database being queried. Writes,
// batch operations, catalog reads and scripts are delegated untouched.
type Executor struct {
	adapters.QueryExecutor

	planner  queryplan.Planner
	policies Policies
	guards   map[string]*Guard
	fallback *Guard
}

// NewExecutor wraps inner so its read statements are checked against policies.
//
// A policy set that enforces nothing returns inner unwrapped: guarding would add
// an EXPLAIN round trip per request without refusing anything. Otherwise inner
// must implement queryplan.Planner, so that a misconfiguration fails at
// startup instead of silently serving traffic unprotected.
func NewExecutor(inner adapters.QueryExecutor, policies Policies) (adapters.QueryExecutor, error) {
	if inner == nil {
		return nil, ErrNilExecutor
	}
	if policies.IsZero() {
		return inner, nil
	}
	planner, ok := inner.(queryplan.Planner)
	if !ok {
		return nil, ErrPlannerUnsupported
	}
	guards := make(map[string]*Guard, len(policies.Databases))
	for alias, policy := range policies.Databases {
		guards[strings.ToLower(alias)] = New(policy)
	}
	return &Executor{
		QueryExecutor: inner,
		planner:       planner,
		policies:      policies,
		guards:        guards,
		fallback:      New(policies.Default),
	}, nil
}

// Query plans SQL on the default connection before running it.
func (e *Executor) Query(SQL string, params ...interface{}) adapters.Scanner {
	plan, err := e.planner.Explain(SQL, params...)
	if err := e.verify("", plan, err); err != nil {
		return &scanner.PrestScanner{Error: err}
	}
	return e.QueryExecutor.Query(SQL, params...)
}

// QueryCtx plans SQL against the policy of the database named in ctx.
func (e *Executor) QueryCtx(ctx context.Context, SQL string, params ...interface{}) adapters.Scanner {
	plan, err := e.planner.ExplainCtx(ctx, SQL, params...)
	if err := e.verify(databaseFromContext(ctx), plan, err); err != nil {
		return &scanner.PrestScanner{Error: err}
	}
	return e.QueryExecutor.QueryCtx(ctx, SQL, params...)
}

// QueryCount plans the count statement before running it.
func (e *Executor) QueryCount(SQL string, params ...interface{}) adapters.Scanner {
	plan, err := e.planner.Explain(SQL, params...)
	if err := e.verify("", plan, err); err != nil {
		return &scanner.PrestScanner{Error: err}
	}
	return e.QueryExecutor.QueryCount(SQL, params...)
}

// QueryCountCtx plans the count statement before running it.
func (e *Executor) QueryCountCtx(ctx context.Context, SQL string, params ...interface{}) adapters.Scanner {
	plan, err := e.planner.ExplainCtx(ctx, SQL, params...)
	if err := e.verify(databaseFromContext(ctx), plan, err); err != nil {
		return &scanner.PrestScanner{Error: err}
	}
	return e.QueryExecutor.QueryCountCtx(ctx, SQL, params...)
}

// GuardFor returns the guard enforcing the policy of a database alias.
func (e *Executor) GuardFor(alias string) *Guard {
	if guard, ok := e.guards[strings.ToLower(alias)]; ok {
		return guard
	}
	return e.fallback
}

// verify turns a planning attempt into a decision. A planning failure blocks the
// query: a statement the engine cannot plan would not execute either, and
// running it anyway would defeat the guard.
//
// A planner that reports neither plan nor error blocks the query for the same
// reason. Guard.Check accepts a nil plan by design, leaving the call here as the
// single place that decides what "no plan" means.
func (e *Executor) verify(database string, plan *queryplan.Node, err error) error {
	if err != nil {
		return fmt.Errorf("query guard could not plan the query: %w", err)
	}
	if plan == nil {
		return fmt.Errorf("query guard could not plan the query: %w", ErrNoQueryPlan)
	}
	if err := e.GuardFor(database).Check(plan); err != nil {
		logRejection(database, err, plan)
		return err
	}
	return nil
}

// databaseFromContext reads the database alias attached to the request.
func databaseFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	name, _ := ctx.Value(pctx.DBNameKey).(string)
	return name
}

// logRejection records the refused plan. The statement text is deliberately not
// logged: it carries user supplied values. The plan shape is enough to tell
// which relation and access path triggered the rule.
func logRejection(database string, err error, plan *queryplan.Node) {
	rejection, ok := Rejection(err)
	if !ok {
		return
	}
	slog.Warn("query rejected by Query Guard",
		"database", database,
		"rule", rejection.Rule,
		"reason", rejection.Reason,
		"estimated_cost", plan.TotalCost,
		"estimated_rows", plan.Rows,
		"plan", summarize(plan))
}

// summarize renders a plan as a compact "NodeType(relation)" list for logs.
func summarize(plan *queryplan.Node) string {
	var parts []string
	plan.Walk(func(node *queryplan.Node) bool {
		if node.Relation != "" {
			parts = append(parts, fmt.Sprintf("%s(%s)", node.NodeType, node.Relation))
			return true
		}
		parts = append(parts, node.NodeType)
		return true
	})
	return strings.Join(parts, " -> ")
}
