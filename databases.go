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
	requestWhere := postgres.WhereByRequest(r)
	sqlDatabases := statements.Databases
	if requestWhere != "" {
		sqlDatabases = fmt.Sprint(
			statements.DatabasesSelect,
			statements.DatabasesWhere,
			"AND",
			requestWhere,
			statements.DatabasesOrderBy)
	}
	object, err := postgres.Query(sqlDatabases)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}
