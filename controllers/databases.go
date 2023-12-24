package controllers

import (
	"fmt"
	"net/http"
	"strings"
)

// GetDatabases list all (or filter) databases
func (c *Config) GetDatabases(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := c.adapter.WhereByRequest(r, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = c.adapter.DatabaseWhere(requestWhere)

	query, hasCount := c.adapter.DatabaseClause(r)
	sqlDatabases := fmt.Sprint(query, requestWhere)

	distinct, err := c.adapter.DistinctClause(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlDatabases = strings.Replace(sqlDatabases, "SELECT", distinct, 1)
	}

	order, err := c.adapter.OrderByRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = c.adapter.DatabaseOrderBy(order, hasCount)

	sqlDatabases = fmt.Sprint(sqlDatabases, order)

	page, err := c.adapter.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlDatabases = fmt.Sprint(sqlDatabases, " ", page)
	sc := c.adapter.Query(sqlDatabases, values...)
	if sc.Err() != nil {
		http.Error(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	//nolint
	w.Write(sc.Bytes())
}
