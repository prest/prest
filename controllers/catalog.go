package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/prest/prest/v2/adapters"
)

// CatalogHandler serves database, schema, and table listing endpoints.
type CatalogHandler struct {
	catalog      adapters.CatalogQuerier
	builder      adapters.RequestQueryBuilder
	executor     adapters.QueryExecutor
	db           adapters.DatabaseRegistry
	registry     adapters.Registry
	singleDB     bool
	registryMode bool
}

func (h *CatalogHandler) ports(r *http.Request) (catalog adapters.CatalogQuerier, builder adapters.RequestQueryBuilder, executor adapters.QueryExecutor, db adapters.DatabaseRegistry) {
	if a := GetAdapterForRequest(r, nil); a != nil {
		return a, a, a, a
	}
	return h.catalog, h.builder, h.executor, h.db
}

func (h *CatalogHandler) bound(r *http.Request) *CatalogHandler {
	catalog, builder, executor, db := h.ports(r)
	cp := *h
	cp.catalog, cp.builder, cp.executor, cp.db = catalog, builder, executor, db
	return &cp
}

// NewCatalogHandler creates a CatalogHandler.
func NewCatalogHandler(deps Deps) *CatalogHandler {
	return &CatalogHandler{
		catalog:      deps.Catalog,
		builder:      deps.Builder,
		executor:     deps.Executor,
		db:           deps.DB,
		registry:     deps.AdapterRegistry,
		singleDB:     deps.SingleDB,
		registryMode: deps.RegistryMode,
	}
}

// databaseRow is one /databases entry in registry mode; same shape as the
// MCP list_databases tool.
type databaseRow struct {
	Datname      string `json:"datname"`
	Name         string `json:"name"`
	PhysicalName string `json:"physical_name"`
}

// listRegistryDatabases answers /databases from the configured aliases: a
// server catalog query would list databases no alias serves.
func (h *CatalogHandler) listRegistryDatabases(w http.ResponseWriter) {
	aliases := append([]string(nil), h.registry.GetAll()...)
	sort.Strings(aliases)
	rows := make([]databaseRow, 0, len(aliases))
	for _, alias := range aliases {
		var db adapters.DatabaseRegistry
		if a, err := h.registry.Get(alias); err == nil {
			db = a
		}
		rows = append(rows, databaseRow{Datname: alias, Name: alias, PhysicalName: physicalName(db, alias)})
	}
	body, err := json.Marshal(rows)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	//nolint
	w.Write(body)
}

// ListDatabases lists all (or filter) databases.
func (h *CatalogHandler) ListDatabases(w http.ResponseWriter, r *http.Request) {
	if h.registryMode && h.registry != nil {
		h.listRegistryDatabases(w)
		return
	}
	h = h.bound(r)
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
	if err := sc.Err(); err != nil {
		writeStatementError(w, err, "", "", "", "")
		return
	}
	//nolint
	w.Write(sc.Bytes())
}

// ListSchemas lists all (or filter) schemas.
func (h *CatalogHandler) ListSchemas(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
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
	if err := sc.Err(); err != nil {
		writeStatementError(w, err, "", "", "", "")
		return
	}
	//nolint
	w.Write(sc.Bytes())
}

// ListTables lists all (or filter) tables.
func (h *CatalogHandler) ListTables(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
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
	if err := sc.Err(); err != nil {
		writeStatementError(w, err, "", "", "", "")
		return
	}
	w.Write(sc.Bytes())
}

// ListTablesByDatabaseAndSchema lists tables for a database and schema.
func (h *CatalogHandler) ListTablesByDatabaseAndSchema(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]

	if err := validateDatabase(database, h.db, h.singleDB); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateSchema(r, schema); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
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
	if err := sc.Err(); err != nil {
		writeStatementError(w, err, schema, "", "", "")
		return
	}
	w.Write(sc.Bytes())
}
