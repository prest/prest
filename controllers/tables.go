package controllers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetTables list all (or filter) tables
func GetTables(w http.ResponseWriter, r *http.Request) {
	requestWhere := postgres.WhereByRequest(r)
	sqlTables := statements.Tables
	if requestWhere != "" {
		sqlTables = fmt.Sprint(
			statements.TablesSelect,
			statements.TablesWhere,
			" AND ",
			requestWhere,
			statements.TablesOrderBy)
	}

	object, err := postgres.Query(sqlTables)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}
