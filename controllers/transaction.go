package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/transactions"
)

type TransactionHandler struct {
	manager  *transactions.Manager
	registry adapters.Registry
}

func NewTransactionHandler(manager *transactions.Manager, registry adapters.Registry) *TransactionHandler {
	return &TransactionHandler{
		manager:  manager,
		registry: registry,
	}
}

func (h *TransactionHandler) Start(w http.ResponseWriter, r *http.Request) {
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]

	if !validatePathSegments(database, schema) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	txID, err := h.manager.Start(r.Context(), database, schema)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"tx": txID})
}

func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]

	if !validatePathSegments(database, schema) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	txs, err := h.manager.List(r.Context(), database, schema)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if txs == nil {
		txs = []transactions.TxInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(txs)
}

func (h *TransactionHandler) Status(w http.ResponseWriter, r *http.Request) {
	vars := pathVars(r)
	txID := vars["txID"]

	info, err := h.manager.Status(r.Context(), txID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (h *TransactionHandler) Commit(w http.ResponseWriter, r *http.Request) {
	vars := pathVars(r)
	txID := vars["txID"]

	if err := h.manager.Commit(r.Context(), txID, h.registry); err != nil {
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "committed"})
}

func (h *TransactionHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	vars := pathVars(r)
	txID := vars["txID"]

	if err := h.manager.Rollback(r.Context(), txID); err != nil {
		jsonError(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "rolled_back"})
}
