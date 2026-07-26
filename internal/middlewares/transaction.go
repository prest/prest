package middlewares

import (
	"context"
	"net/http"

	pctx "github.com/prest/prest/v2/internal/contextkeys"
	"github.com/prest/prest/v2/internal/transactions"

	"github.com/urfave/negroni/v3"
)

const transactionHeader = "Authorization-Transaction"

func TransactionMiddleware(manager *transactions.Manager) negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		txID := r.Header.Get(transactionHeader)
		if txID == "" {
			next(w, r)
			return
		}

		managedTx, ok := manager.Get(r.Context(), txID)
		if !ok {
			http.Error(w, `{"error":"transaction not found or not pending"}`, http.StatusConflict)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, pctx.TxKey, managedTx.ID)
		next(w, r.WithContext(ctx))
	})
}
