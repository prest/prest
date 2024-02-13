package controllers

import (
	"fmt"
	"net/http"
	"strings"
)

// GetDatabases retrieves the list of databases based on the provided HTTP request.
// It applies filters, distinct clause, order by clause, pagination, and executes the query.
// The resulting list of databases is written as JSON to the HTTP response writer.
func (c *Config) GetDatabases(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := c.adapter.WhereByRequest(r, 1)
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = c.adapter.DatabaseWhere(requestWhere)

	query, hasCount := c.adapter.DatabaseClause(r)
	dbsQuery := fmt.Sprint(query, requestWhere)

	distinct, err := c.adapter.DistinctClause(r)
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		dbsQuery = strings.Replace(dbsQuery, "SELECT", distinct, 1)
	}

	order, err := c.adapter.OrderByRequest(r)
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = c.adapter.DatabaseOrderBy(order, hasCount)

	dbsQuery = fmt.Sprint(dbsQuery, order)

	page, err := c.adapter.PaginateIfPossible(r)
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	dbsQuery = fmt.Sprint(dbsQuery, " ", page)
	sc := c.adapter.QueryCtx(r.Context(), dbsQuery, values...)
	err = sc.Err()
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	JSONWrite(w, string(sc.Bytes()), http.StatusOK)
}
