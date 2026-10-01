package adapters

import "net/http"

// GroupByBinder is optional. Select uses it when the builder implements it
// so a HAVING literal is a bound parameter. GroupByClause stays a string.
type GroupByBinder interface {
	GroupByClauseValues(r *http.Request, initialPlaceholderID int) (clause string, values []any)
}
