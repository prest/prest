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

func validateDatabase(database string, registry adapters.DatabaseRegistry) error {
	if registry != nil && !registry.IsRegistered(database) {
		return fmt.Errorf("database not registered: %v", database)
	}
	return nil
}

// errSchemaNotInDatabase rejects a {schema} the request's adapter does not
// serve, so a MySQL alias cannot reach another physical database.
var errSchemaNotInDatabase = errors.New("schema not served by this database")

// validateSchema checks {schema} against the adapter selected for the request.
// Adapters implementing neither adapters.DatabaseSchemaScoper nor
// adapters.SchemaScoper accept any schema.
func validateSchema(r *http.Request, database, schema string) error {
	return checkSchemaScope(GetAdapterForRequest(r, nil), database, schema)
}

// checkSchemaScope prefers the database-aware scoper and falls back to the
// one-argument SchemaScoper.
func checkSchemaScope(a adapters.Adapter, database, schema string) error {
	if scoper, ok := a.(adapters.DatabaseSchemaScoper); ok {
		if !scoper.AllowsDatabaseSchema(database, schema) {
			return errSchemaNotInDatabase
		}
		return nil
	}
	if scoper, ok := a.(adapters.SchemaScoper); ok && !scoper.AllowsSchema(schema) {
		return errSchemaNotInDatabase
	}
	return nil
}

// registryAliases lists the databases this server serves: registry aliases (or
// pg.database plus pg.allowed_databases via the adapter), falling back to the
// default database.
func registryAliases(db adapters.DatabaseRegistry, defaultDB string) []string {
	if db != nil {
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
