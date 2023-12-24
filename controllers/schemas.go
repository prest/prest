package controllers

import (
	"fmt"
	"net/http"
	"strings"

	pctx "github.com/prest/prest/context"
)

// GetSchemas list all (or filter) schemas
func (c *Config) GetSchemas(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := c.adapter.WhereByRequest(r, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemas, hasCount := c.adapter.SchemaClause(r)

	if requestWhere != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, " WHERE ", requestWhere)
	}

	distinct, err := c.adapter.DistinctClause(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlSchemas = strings.Replace(sqlSchemas, "SELECT", distinct, 1)
	}

	order, err := c.adapter.OrderByRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = c.adapter.SchemaOrderBy(order, hasCount)

	page, err := c.adapter.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := pctx.WithTimeout(r.Context())
	defer cancel()

	sqlSchemas = fmt.Sprint(sqlSchemas, order, " ", page)
	sc := c.adapter.QueryCtx(ctx, sqlSchemas, values...)
	if sc.Err() != nil {
		http.Error(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	//nolint
	w.Write(sc.Bytes())
}
