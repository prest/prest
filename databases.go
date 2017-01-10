package controllers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetDatabases list all (or filter) databases
func GetDatabases(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := postgres.DatabaseClause(r)
	sqlDatabases := fmt.Sprint(query, statements.DatabasesWhere)

	if requestWhere != "" {
		sqlDatabases = fmt.Sprint(sqlDatabases, " AND ", requestWhere)
	}

	order, _ := postgres.OrderByRequest(r)
	if order != "" {
		sqlDatabases = fmt.Sprint(sqlDatabases, order)
	} else {
		sqlDatabases = fmt.Sprint(sqlDatabases, statements.DatabasesOrderBy)
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, "Paging error", http.StatusBadRequest)
		return
	}

	sqlDatabases = fmt.Sprint(sqlDatabases, " ", page)

	object, err := postgres.Query(sqlDatabases, values...)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}
