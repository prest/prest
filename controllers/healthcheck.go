package controllers

import (
	"context"
	"net/http"

	"github.com/prest/prest/adapters"
	pctx "github.com/prest/prest/context"
	"github.com/structy/log"
)

type CheckList []func(context.Context, adapters.Adapter) error

var DefaultCheckList = CheckList{
	CheckDBHealth,
}

// CheckDBHealth checks the health of the database by executing a simple query.
// It takes a context and an adapter as parameters.
// The adapter is used to establish a connection to the database.
// If the connection is successful, it executes a query and commits the transaction.
// If any error occurs during the process, it returns the error.
func CheckDBHealth(ctx context.Context, adptr adapters.Adapter) error {
	conn, err := adptr.GetTransactionCtx(ctx)
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, ";")
	if err != nil {
		return err
	}
	return conn.Commit()
}

// WrappedHealthCheck is a handler function that performs health checks on a list of checks.
// It takes a CheckList as input and returns an http.HandlerFunc.
//
// Each check in the CheckList is executed, and if any check fails, the handler
// responds with a 503 Service Unavailable status code.
//
// If all checks pass, the handler responds with a 200 OK status code.
func (c *Config) WrappedHealthCheck(checks CheckList) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := pctx.WithTimeout(r.Context())
		defer cancel()

		for _, check := range checks {
			if err := check(ctx, c.adapter); err != nil {
				log.Errorf("could not check DB connection: %v\n", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
