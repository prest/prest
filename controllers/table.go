package controllers

import (
	"fmt"
	"net/http"

	"github.com/prest/prest/v2/adapters"
)

// TableHandler serves table metadata endpoints.
type TableHandler struct {
	executor adapters.QueryExecutor
	db       adapters.DatabaseRegistry
	singleDB bool
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
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if err := validateDatabase(database, h.db, h.singleDB); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !validatePathSegments(database, schema, table) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.ShowTableCtx(ctx, schema, table)
	if sc.Err() != nil {
		errorMessage := fmt.Sprintf("error to execute query, schema error %s", sc.Err())
		jsonError(w, errorMessage, http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}
