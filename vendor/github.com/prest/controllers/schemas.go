package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/prest/adapters/postgres"
	"github.com/prest/statements"
)

// GetSchemas list all (or filter) schemas
func GetSchemas(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemas, hasCount := postgres.SchemaClause(r)

	if requestWhere != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, " WHERE ", requestWhere)
	}

	distinct, err := postgres.DistinctClause(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlSchemas = strings.Replace(sqlSchemas, "SELECT", distinct, 1)
	}

	order, err := postgres.OrderByRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if order != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, order)
	} else if !hasCount {
		sqlSchemas = fmt.Sprint(sqlSchemas, fmt.Sprintf(statements.SchemasOrderBy, statements.FieldSchemaName))
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemas = fmt.Sprint(sqlSchemas, " ", page)
	sc := postgres.Query(sqlSchemas, values...)
	if sc.Err() != nil {
		http.Error(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}
