package adapters

import "net/http"

// GroupByBinder is optional. GroupByFromRequest uses it when the builder
// implements it so a HAVING literal is a bound parameter. GroupByClause stays a string.
type GroupByBinder interface {
	GroupByClauseValues(r *http.Request, initialPlaceholderID int) (clause string, values []any)
}

// GroupByFromRequest uses GroupByClauseValues when the builder implements
// GroupByBinder. A string-only builder returns GroupByClause and a nil slice.
func GroupByFromRequest(builder RequestQueryBuilder, r *http.Request, initialPlaceholderID int) (clause string, values []any) {
	if binder, ok := builder.(GroupByBinder); ok {
		return binder.GroupByClauseValues(r, initialPlaceholderID)
	}
	return builder.GroupByClause(r), nil
}
