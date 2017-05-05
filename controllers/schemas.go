package controllers

import (
	"fmt"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/helpers"
	"github.com/nuveo/prest/statements"
)

// GetSchemas list all (or filter) schemas
func GetSchemas(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		helpers.ErrorHandler(w, err)
		return
	}

	sqlSchemas, hasCount := postgres.SchemaClause(r)

	if requestWhere != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, " WHERE ", requestWhere)
	}

	order, err := postgres.OrderByRequest(r)
	if err != nil {
		helpers.ErrorHandler(w, err)
		return
	}

	if order != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, order)
	} else if !hasCount {
		sqlSchemas = fmt.Sprint(sqlSchemas, fmt.Sprintf(statements.SchemasOrderBy, statements.FieldSchemaName))
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		helpers.ErrorHandler(w, err)
		return
	}

	sqlSchemas = fmt.Sprint(sqlSchemas, " ", page)
	object, err := postgres.Query(sqlSchemas, values...)
	if err != nil {
		helpers.ErrorHandler(w, err)
		return
	}

	w.Write(object)
}
