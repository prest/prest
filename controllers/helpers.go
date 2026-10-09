package controllers

import (
	"context"
	"errors"
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

// errSchemaNotInDatabase rejects a {schema} the request's adapter does not
// serve, so a MySQL alias cannot reach another physical database.
var errSchemaNotInDatabase = errors.New("schema not served by this database")

// validateSchema checks {schema} against the adapter selected for the request.
// Adapters that do not implement adapters.SchemaScoper accept any schema.
func validateSchema(r *http.Request, schema string) error {
	return checkSchemaScope(GetAdapterForRequest(r, nil), schema)
}

func checkSchemaScope(a adapters.Adapter, schema string) error {
	if scoper, ok := a.(adapters.SchemaScoper); ok && !scoper.AllowsSchema(schema) {
		return errSchemaNotInDatabase
	}
	return nil
}

// registryAliases lists the database aliases a server exposes: the default
// database in single-DB mode, otherwise the registry's aliases, falling back
// to the default database.
func registryAliases(db adapters.DatabaseRegistry, singleDB bool, defaultDB string) []string {
	if !singleDB && db != nil {
		if aliases := uniqueStrings(db.Aliases()); len(aliases) > 0 {
			return aliases
		}
	}
	if defaultDB == "" {
		return nil
	}
	return []string{defaultDB}
}

// physicalName resolves an alias to the database name on the server.
func physicalName(db adapters.DatabaseRegistry, alias string) string {
	if db == nil {
		return alias
	}
	return db.PhysicalName(alias)
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
