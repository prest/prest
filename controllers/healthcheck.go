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
