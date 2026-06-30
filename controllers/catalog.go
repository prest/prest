package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/prest/prest/v2/adapters"
)

// CatalogHandler serves database, schema, and table listing endpoints.
type CatalogHandler struct {
	catalog  adapters.CatalogQuerier
	builder  adapters.RequestQueryBuilder
	executor adapters.QueryExecutor
	db       adapters.DatabaseRegistry
	singleDB bool
}

// NewCatalogHandler creates a CatalogHandler.
func NewCatalogHandler(deps Deps) *CatalogHandler {
	return &CatalogHandler{
		catalog:  deps.Catalog,
		builder:  deps.Builder,
		executor: deps.Executor,
		db:       deps.DB,
		singleDB: deps.SingleDB,
	}
}

// ListDatabases lists all (or filter) databases.
func (h *CatalogHandler) ListDatabases(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := h.builder.WhereByRequest(r, 1)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = h.catalog.DatabaseWhere(requestWhere)

	query, hasCount := h.catalog.DatabaseClause(r)
	sqlDatabases := fmt.Sprint(query, requestWhere)

	distinct, err := h.builder.DistinctClause(r)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlDatabases = strings.Replace(sqlDatabases, "SELECT", distinct, 1)
	}

	order, err := h.builder.OrderByRequest(r)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = h.catalog.DatabaseOrderBy(order, hasCount)

	sqlDatabases = fmt.Sprint(sqlDatabases, order)

	page, err := h.builder.PaginateIfPossible(r)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlDatabases = fmt.Sprint(sqlDatabases, " ", page)
	sc := h.executor.Query(sqlDatabases, values...)
	if sc.Err() != nil {
		jsonError(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	//nolint
	w.Write(sc.Bytes())
}

// ListSchemas lists all (or filter) schemas.
func (h *CatalogHandler) ListSchemas(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := h.builder.WhereByRequest(r, 1)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemas, hasCount := h.catalog.SchemaClause(r)

	if requestWhere != "" {
		sqlSchemas = fmt.Sprint(sqlSchemas, " WHERE ", requestWhere)
	}

	distinct, err := h.builder.DistinctClause(r)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlSchemas = strings.Replace(sqlSchemas, "SELECT", distinct, 1)
	}

	order, err := h.builder.OrderByRequest(r)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = h.catalog.SchemaOrderBy(order, hasCount)

	page, err := h.builder.PaginateIfPossible(r)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemas = fmt.Sprint(sqlSchemas, order, " ", page)
	sc := h.executor.Query(sqlSchemas, values...)
	if sc.Err() != nil {
		jsonError(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	//nolint
	w.Write(sc.Bytes())
}

// ListTables lists all (or filter) tables.
func (h *CatalogHandler) ListTables(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := h.builder.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = h.catalog.TableWhere(requestWhere)

	order, err := h.builder.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = h.catalog.TableOrderBy(order)

	sqlTables := h.catalog.TableClause()

	distinct, err := h.builder.DistinctClause(r)
	if err != nil {
		err = fmt.Errorf("could not perform Distinct: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlTables = strings.Replace(sqlTables, "SELECT", distinct, 1)
	}

	page, err := h.builder.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlTables = strings.Join([]string{sqlTables, requestWhere, order, page}, " ")

	sc := h.executor.Query(sqlTables, values...)
	if sc.Err() != nil {
		jsonError(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}

// ListTablesByDatabaseAndSchema lists tables for a database and schema.
func (h *CatalogHandler) ListTablesByDatabaseAndSchema(w http.ResponseWriter, r *http.Request) {
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]

	if err := validateDatabase(database, h.db, h.singleDB); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !validatePathSegments(database, schema) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	requestWhere, values, err := h.builder.WhereByRequest(r, 3)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = h.catalog.SchemaTablesWhere(requestWhere)

	sqlSchemaTables := h.catalog.SchemaTablesClause()

	order, err := h.builder.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = h.catalog.SchemaTablesOrderBy(order)

	page, err := h.builder.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemaTables = fmt.Sprint(sqlSchemaTables, requestWhere, order, " ", page)

	valuesAux := make([]interface{}, 0)
	valuesAux = append(valuesAux, database, schema)
	valuesAux = append(valuesAux, values...)

	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.QueryCtx(ctx, sqlSchemaTables, valuesAux...)
	if sc.Err() != nil {
		jsonError(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}
