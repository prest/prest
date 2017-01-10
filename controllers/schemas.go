package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetSchemas list all (or filter) schemas
func GetSchemas(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemas := postgres.SchemaClause(r)

	if requestWhere != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, " WHERE ", requestWhere)
	}

	if strings.Contains(sqlSchemas, "COUNT") {
		sqlSchemas = fmt.Sprint(sqlSchemas, statements.SchemasGroupBy)
	}

	order, _ := postgres.OrderByRequest(r)
	if order != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, order)
	} else {
		sqlSchemas = fmt.Sprint(sqlSchemas, statements.SchemasOrderBy)
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, "Paging error", http.StatusBadRequest)
		return
	}

	sqlSchemas = fmt.Sprint(sqlSchemas, " ", page)

	object, err := postgres.Query(sqlSchemas, values...)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}
