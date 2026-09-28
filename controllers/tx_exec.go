package controllers

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/prest/prest/v2/adapters"
)

// transactionHeader carries the id of an open transaction from one request to
// the next. A request without it is not in a transaction.
const transactionHeader = "X-Prest-Transaction"

// TxExecutor is the narrow slice of the adapter that can run a write inside a
// caller-supplied transaction.
//
// The postgres adapter already declares InsertWithTransaction,
// UpdateWithTransaction and DeleteWithTransaction on adapters.Adapter, so this
// is an interface it satisfies rather than a new implementation. Declaring the
// three methods here rather than reusing adapters.Adapter keeps the CRUD
// handler's dependency to what it actually calls.
//
// Select is deliberately absent. There is no QueryWithTransaction, and adding
// one is a larger change to the read path than this issue asks for.
type TxExecutor interface {
	InsertWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) adapters.Scanner
	UpdateWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) adapters.Scanner
	DeleteWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) adapters.Scanner
}

// lookupTx resolves the transaction header for a request.
//
// It returns (nil, false) when the request is not in a transaction, which is
// the ordinary case and must fall through to the normal path. A header naming
// an unknown or already-closed transaction writes a 410 and returns
// (nil, false): callers treat a nil tx as "do not run this statement", so a
// stale header fails the request instead of silently executing outside the
// transaction and leaving the client believing its writes were atomic.
func (h *CRUDHandler) lookupTx(w http.ResponseWriter, r *http.Request) *sql.Tx {
	if r.Header.Get(transactionHeader) == "" {
		return nil
	}
	if h.txHandler == nil || h.txExec == nil {
		writeTransactionError(w, http.StatusNotImplemented,
			"this adapter does not support transactions")
		return nil
	}
	tx, ok := h.txHandler.Lookup(r)
	if !ok {
		writeTransactionError(w, http.StatusGone, "transaction not found or no longer open")
	}
	return tx
}

// runInsert executes an INSERT, inside the request's transaction when the
// X-Prest-Transaction header names one. Returns false if the request was
// rejected and a response has already been written.
func (h *CRUDHandler) runInsert(ctx context.Context, w http.ResponseWriter, r *http.Request, query string, params ...interface{}) (adapters.Scanner, bool) {
	if r.Header.Get(transactionHeader) == "" {
		return h.executor.InsertCtx(ctx, query, params...), true
	}
	tx := h.lookupTx(w, r)
	if tx == nil {
		return nil, false
	}
	return h.txExec.InsertWithTransaction(tx, query, params...), true
}

// runUpdate executes an UPDATE, inside the request's transaction when the
// X-Prest-Transaction header names one. Returns false if the request was
// rejected and a response has already been written.
func (h *CRUDHandler) runUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request, query string, params ...interface{}) (adapters.Scanner, bool) {
	if r.Header.Get(transactionHeader) == "" {
		return h.executor.UpdateCtx(ctx, query, params...), true
	}
	tx := h.lookupTx(w, r)
	if tx == nil {
		return nil, false
	}
	return h.txExec.UpdateWithTransaction(tx, query, params...), true
}

// runDelete executes a DELETE, inside the request's transaction when the
// X-Prest-Transaction header names one. Returns false if the request was
// rejected and a response has already been written.
func (h *CRUDHandler) runDelete(ctx context.Context, w http.ResponseWriter, r *http.Request, query string, params ...interface{}) (adapters.Scanner, bool) {
	if r.Header.Get(transactionHeader) == "" {
		return h.executor.DeleteCtx(ctx, query, params...), true
	}
	tx := h.lookupTx(w, r)
	if tx == nil {
		return nil, false
	}
	return h.txExec.DeleteWithTransaction(tx, query, params...), true
}
