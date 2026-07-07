package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prest/prest/v2/adapters"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/internal/ident"

	"github.com/gorilla/mux"
)

func requestContext(r *http.Request, database string) (context.Context, context.CancelFunc) {
	ctx := context.WithValue(r.Context(), pctx.DBNameKey, database)
	timeout, ok := ctx.Value(pctx.HTTPTimeoutKey).(int)
	if !ok || timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, time.Second*time.Duration(timeout))
}

func validateDatabase(database string, registry adapters.DatabaseRegistry, singleDB bool) error {
	if registry != nil && !registry.IsRegistered(database) {
		return fmt.Errorf("database not registered: %v", database)
	}
	if singleDB && registry != nil && registry.GetDatabase() != database {
		return fmt.Errorf("database not registered: %v", database)
	}
	return nil
}

func validatePathSegments(segments ...string) bool {
	for _, s := range segments {
		if !ident.IsSafeSegment(s) {
			return false
		}
	}
	return true
}

func pathVars(r *http.Request) map[string]string {
	return mux.Vars(r)
}
