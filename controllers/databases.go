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
	requestWhere, values := postgres.WhereByRequest(r, 1)
	sqlDatabases := statements.Databases
	if requestWhere != "" {
		sqlDatabases = fmt.Sprint(
			statements.DatabasesSelect,
			statements.DatabasesWhere,
			" AND ",
			requestWhere,
			statements.DatabasesOrderBy)
	}
	sqlDatabases = fmt.Sprint(sqlDatabases, " ", postgres.PaginateIfPossible(r))
	object, err := postgres.Query(sqlDatabases, values)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}
