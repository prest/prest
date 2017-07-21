package controllers

import (
	"fmt"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetDatabases list all (or filter) databases
func GetDatabases(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query, hasCount := postgres.DatabaseClause(r)
	sqlDatabases := fmt.Sprint(query, statements.DatabasesWhere)

	if requestWhere != "" {
		sqlDatabases = fmt.Sprint(sqlDatabases, " AND ", requestWhere)
	}

	order, err := postgres.OrderByRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if order != "" {
		sqlDatabases = fmt.Sprint(sqlDatabases, order)
	} else if !hasCount {
		sqlDatabases = fmt.Sprint(sqlDatabases, fmt.Sprintf(statements.DatabasesOrderBy, statements.FieldDatabaseName))
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlDatabases = fmt.Sprint(sqlDatabases, " ", page)
	object, err := postgres.Query(ctx, sqlDatabases, values...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(object)
}
