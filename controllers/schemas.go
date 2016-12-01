package controllers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetSchemas list all (or filter) schemas
func GetSchemas(w http.ResponseWriter, r *http.Request) {
	requestWhere := postgres.WhereByRequest(r)
	sqlSchemas := statements.Schemas
	if requestWhere != "" {
		sqlSchemas = fmt.Sprint(
			statements.SchemasSelect,
			" WHERE ",
			requestWhere,
			statements.SchemasOrderBy)
	}
	sqlSchemas = fmt.Sprint(sqlSchemas, " ", postgres.PaginateIfPossible(r))
	object, err := postgres.Query(sqlSchemas)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}
