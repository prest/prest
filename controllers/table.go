package controllers

import (
	"net/http"

	"github.com/prest/prest/v2/adapters"
)

// TableHandler serves table metadata endpoints.
type TableHandler struct {
	executor adapters.QueryExecutor
	db       adapters.DatabaseRegistry
	singleDB bool
}

func (h *TableHandler) ports(r *http.Request) (executor adapters.QueryExecutor, db adapters.DatabaseRegistry) {
	if a := GetAdapterForRequest(r, nil); a != nil {
		return a, a
	}
	return h.executor, h.db
}

func (h *TableHandler) bound(r *http.Request) *TableHandler {
	executor, db := h.ports(r)
	cp := *h
	cp.executor, cp.db = executor, db
	return &cp
}

// NewTableHandler creates a TableHandler.
func NewTableHandler(executor adapters.QueryExecutor, db adapters.DatabaseRegistry, singleDB bool) *TableHandler {
	return &TableHandler{
		executor: executor,
		db:       db,
		singleDB: singleDB,
	}
}

// Show returns information about a table.
func (h *TableHandler) Show(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if err := validateDatabase(database, h.db, h.singleDB); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateSchema(r, schema); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	if !validatePathSegments(database, schema, table) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.ShowTableCtx(ctx, schema, table)
	if err := sc.Err(); err != nil {
		const prefix = "error to execute query, schema error "
		writeStatementError(w, err, schema, table, prefix, prefix)
		return
	}
	w.Write(sc.Bytes())
}
