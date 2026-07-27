package timescaledb

import (
	"context"
	"errors"

	"github.com/prest/prest/v2/adapters/queryplan"
)

// ErrNoQueryPlanner is returned when the wrapped adapter cannot explain
// statements. TimescaleDB always wraps postgres, which can, so this only guards
// against an adapter injected for tests.
var ErrNoQueryPlanner = errors.New("wrapped adapter does not support query plans")

// Explain delegates planning to the wrapped adapter. The embedded field is the
// adapters.Adapter interface, which does not carry the optional planner, so the
// delegation is explicit.
func (a *Adapter) Explain(SQL string, params ...interface{}) (*queryplan.Node, error) {
	planner, ok := a.Adapter.(queryplan.Planner)
	if !ok {
		return nil, ErrNoQueryPlanner
	}
	return planner.Explain(SQL, params...)
}

// ExplainCtx delegates planning to the wrapped adapter.
func (a *Adapter) ExplainCtx(ctx context.Context, SQL string, params ...interface{}) (*queryplan.Node, error) {
	planner, ok := a.Adapter.(queryplan.Planner)
	if !ok {
		return nil, ErrNoQueryPlanner
	}
	return planner.ExplainCtx(ctx, SQL, params...)
}
