package controllers

import (
	"context"
	"net/http"

	"github.com/prest/prest/adapters/postgres"
	pctx "github.com/prest/prest/context"
	"github.com/structy/log"
)

type CheckList []func(context.Context) error

var DefaultCheckList = CheckList{
	CheckDBHealth,
}

// todo: detach postgres from here
// this will allow us to use other databases
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
		ctx, cancel := pctx.WithTimeout(r.Context())
		defer cancel()

		for _, check := range checks {
			if err := check(ctx); err != nil {
				log.Errorf("could not check DB connection: %v\n", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}
