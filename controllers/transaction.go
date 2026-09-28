package controllers

import (
	"errors"
	"net/http"

	"database/sql"

	"github.com/prest/prest/v2/internal/logsafe"
	"github.com/prest/prest/v2/transactions"

	"log/slog"
)

// TransactionHandler serves the transaction endpoints.
//
// A transaction is opened with POST and referenced by the returned ID on
// subsequent requests via the X-Prest-Transaction header. Commit and rollback
// take the ID in the path, so a client can abandon a transaction without being
// able to commit it by accident.
type TransactionHandler struct {
	mgr *transactions.Manager
	db  transactions.Beginner
}

func NewTransactionHandler(db transactions.Beginner) *TransactionHandler {
	return &TransactionHandler{mgr: transactions.New(), db: db}
}

// Handler routes by method so the three verbs can share one path.
func (h *TransactionHandler) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.Begin(w, r)
		case http.MethodDelete:
			h.Rollback(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

type transactionResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

// Begin opens a transaction and returns its ID.
func (h *TransactionHandler) Begin(w http.ResponseWriter, r *http.Request) {
	id, err := h.mgr.BeginWith(r.Context(), h.db)
	if err != nil {
		slog.Error("could not begin transaction", "err", logsafe.Error(err))
		writeTransactionError(w, http.StatusInternalServerError,
			"could not begin transaction")
		return
	}
	writeJSONStatus(w, http.StatusCreated, transactionResponse{TransactionID: id, Status: "open"})
}

// Commit commits the transaction named in the mux var "id".
func (h *TransactionHandler) Commit(w http.ResponseWriter, r *http.Request) {
	h.finish(w, r, true)
}

// Rollback rolls back the transaction named in the mux var "id".
func (h *TransactionHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	h.finish(w, r, false)
}

func (h *TransactionHandler) finish(w http.ResponseWriter, r *http.Request, commit bool) {
	id := r.PathValue("id")
	var err error
	if commit {
		err = h.mgr.Commit(id)
	} else {
		err = h.mgr.Rollback(id)
	}
	switch {
	case errors.Is(err, transactions.ErrNotFound):
		// 410 rather than 404: the resource existed and is gone, and a client
		// retrying with a stale ID should not expect to create anything.
		writeTransactionError(w, http.StatusGone, "unknown or already-closed transaction")
	case err != nil:
		slog.Error("could not close transaction", "err", logsafe.Error(err))
		writeTransactionError(w, http.StatusInternalServerError, "could not close transaction")
	default:
		status := "committed"
		if !commit {
			status = "rolled back"
		}
		writeJSONStatus(w, http.StatusOK, transactionResponse{TransactionID: id, Status: status})
	}
}

// Lookup returns the transaction named in the X-Prest-Transaction header, or
// reports that the request is not part of a transaction.
//
// It is exported so the CRUD path can join an open transaction rather than
// reimplementing the header lookup.
func (h *TransactionHandler) Lookup(r *http.Request) (*sql.Tx, bool) {
	id := r.Header.Get("X-Prest-Transaction")
	if id == "" {
		return nil, false
	}
	tx, ok := h.mgr.Get(id)
	if !ok {
		return nil, false
	}
	return tx, true
}

func writeTransactionError(w http.ResponseWriter, code int, msg string) {
	writeJSONStatus(w, code, map[string]string{"error": msg})
}
