package controllers

import (
	"context"
	"net/http"
	"time"

	"github.com/prest/prest/adapters/postgres"
	pctx "github.com/prest/prest/context"
	"github.com/structy/log"
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
				log.Errorf("could not check DB connection: %v\n", err)
				http.ResponseWriter.WriteHeader(w, http.StatusServiceUnavailable)
				return
			}
		}
		http.ResponseWriter.WriteHeader(w, http.StatusOK)
	}
}
