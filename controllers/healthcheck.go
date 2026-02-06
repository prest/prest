package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/prest/prest/v2/adapters/postgres"
	pctx "github.com/prest/prest/v2/context"

	"log/slog"
)

type CheckList []func(context.Context) error

var DefaultCheckList = CheckList{
	CheckDBHealth,
}

func CheckDBHealth(ctx context.Context) error {
	conn, err := postgres.Get()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, ";")
	return err
}

func WrappedHealthCheck(checks CheckList) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		timeout, _ := r.Context().Value(pctx.HTTPTimeoutKey).(int)
		ctx, cancel := context.WithTimeout(
			r.Context(), time.Second*time.Duration(timeout))
		defer cancel()
		for _, check := range checks {
			if err := check(ctx); err != nil {
				slog.Error("could not check DB connection", "err", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
