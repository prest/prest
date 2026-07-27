package queryplan

import (
	"context"
	"errors"
)

// ErrEmptyPlan is returned when the database answers an EXPLAIN with no plan.
var ErrEmptyPlan = errors.New("database returned an empty query plan")

// ErrNoExplainer is returned when a planner was built without a way to reach the
// database, which makes it unable to plan anything.
var ErrNoExplainer = errors.New("query planner requires an explainer")

// Planner produces the execution plan of a statement without running it.
//
// It is an optional adapter capability: engines that cannot explain statements
// simply do not provide one.
type Planner interface {
	Explain(SQL string, params ...interface{}) (*Node, error)
	ExplainCtx(ctx context.Context, SQL string, params ...interface{}) (*Node, error)
}

// Explainer runs an EXPLAIN statement and returns its raw payload.
//
// It is the only capability a planner needs from an adapter. Handing back bytes
// rather than a prepared statement keeps connection selection and statement
// lifecycle inside the adapter, which owns (and caches) its statements.
type Explainer interface {
	ExplainRow(ctx context.Context, SQL string, params ...interface{}) ([]byte, error)
}
