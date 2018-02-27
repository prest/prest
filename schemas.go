package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/prest/config"
)

// GetSchemas list all (or filter) schemas
func GetSchemas(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := config.PrestConf.Adapter.WhereByRequest(r, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemas, hasCount := config.PrestConf.Adapter.SchemaClause(r)

	if requestWhere != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, " WHERE ", requestWhere)
	}

	distinct, err := config.PrestConf.Adapter.DistinctClause(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlSchemas = strings.Replace(sqlSchemas, "SELECT", distinct, 1)
	}

	order, err := config.PrestConf.Adapter.OrderByRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = config.PrestConf.Adapter.SchemaOrderBy(order, hasCount)

	page, err := config.PrestConf.Adapter.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemas = fmt.Sprint(sqlSchemas, order, " ", page)
	sc := config.PrestConf.Adapter.Query(sqlSchemas, values...)
	if sc.Err() != nil {
		http.Error(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}
