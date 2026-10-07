package timescaledb

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/postgres"
)

// ErrNotTimescaleDBAdapter is returned when an adapter does not support TimescaleDB connection helpers.
var ErrNotTimescaleDBAdapter = errors.New("adapter is not timescaledb")

// Connect implements adapters.DatabaseConnector by delegating to the embedded postgres adapter.
// Required because embedding adapters.Adapter (interface) does not promote optional connector methods.
func (a *Adapter) Connect() error {
	c, ok := a.Adapter.(adapters.DatabaseConnector)
	if !ok {
		return ErrNotTimescaleDBAdapter
	}
	return c.Connect()
}

// DB implements adapters.DatabaseAccessor by delegating to the embedded postgres adapter.
func (a *Adapter) DB() (*sqlx.DB, error) {
	d, ok := a.Adapter.(adapters.DatabaseAccessor)
	if !ok {
		return nil, ErrNotTimescaleDBAdapter
	}
	return d.DB()
}

// queryRegistry returns the wrapped adapter's QueryRegistry. Embedding the
// adapters.Adapter interface does not promote optional ports.
func (a *Adapter) queryRegistry() (adapters.QueryRegistry, error) {
	r, ok := a.Adapter.(adapters.QueryRegistry)
	if !ok {
		return nil, ErrNotTimescaleDBAdapter
	}
	return r, nil
}

// ListQueries implements adapters.QueryRegistry by delegating to the embedded postgres adapter.
func (a *Adapter) ListQueries(ctx context.Context, databaseAlias, location string) ([]adapters.StoredQuery, error) {
	r, err := a.queryRegistry()
	if err != nil {
		return nil, err
	}
	return r.ListQueries(ctx, databaseAlias, location)
}

// GetQuery implements adapters.QueryRegistry by delegating to the embedded postgres adapter.
func (a *Adapter) GetQuery(ctx context.Context, databaseAlias, location, name string) (adapters.StoredQuery, error) {
	r, err := a.queryRegistry()
	if err != nil {
		return adapters.StoredQuery{}, err
	}
	return r.GetQuery(ctx, databaseAlias, location, name)
}

// UpsertQuery implements adapters.QueryRegistry by delegating to the embedded postgres adapter.
func (a *Adapter) UpsertQuery(ctx context.Context, query adapters.StoredQuery) error {
	r, err := a.queryRegistry()
	if err != nil {
		return err
	}
	return r.UpsertQuery(ctx, query)
}

// DeleteQuery implements adapters.QueryRegistry by delegating to the embedded postgres adapter.
func (a *Adapter) DeleteQuery(ctx context.Context, databaseAlias, location, name string) error {
	r, err := a.queryRegistry()
	if err != nil {
		return err
	}
	return r.DeleteQuery(ctx, databaseAlias, location, name)
}

// ImportFromFilesystem implements adapters.QueryRegistry by delegating to the embedded postgres adapter.
func (a *Adapter) ImportFromFilesystem(ctx context.Context, queriesPath, policy string) (adapters.ImportReport, error) {
	r, err := a.queryRegistry()
	if err != nil {
		return adapters.ImportReport{}, err
	}
	return r.ImportFromFilesystem(ctx, queriesPath, policy)
}

// ScriptPermissions implements adapters.ScriptPermissionsChecker by delegating to the
// embedded postgres adapter; without one it denies, like middlewares.denyAllScriptPerms.
func (a *Adapter) ScriptPermissions(ctx context.Context, databaseAlias, location, name, op, userName string) bool {
	p, ok := a.Adapter.(adapters.ScriptPermissionsChecker)
	return ok && p.ScriptPermissions(ctx, databaseAlias, location, name, op, userName)
}

// Connect initializes the TimescaleDB adapter connection pool and verifies TimescaleDB is available.
func Connect(a adapters.Adapter) error {
	c, ok := a.(adapters.DatabaseConnector)
	if !ok {
		return ErrNotTimescaleDBAdapter
	}
	if err := c.Connect(); err != nil {
		return err
	}
	// Verify TimescaleDB extension is available
	return verifyTimescaleDB(a)
}

// verifyTimescaleDB checks that the timescaledb extension is available in the connected database.
func verifyTimescaleDB(a adapters.Adapter) error {
	d, ok := a.(adapters.DatabaseAccessor)
	if !ok {
		return ErrNotTimescaleDBAdapter
	}
	db, err := d.DB()
	if err != nil {
		return err
	}
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='timescaledb')").Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("timescaledb extension not found; connected database does not have timescaledb installed")
	}
	return nil
}

// IsTimescaleDB checks if the connected adapter is running against TimescaleDB.
// This involves querying the database to check for the timescaledb extension.
func IsTimescaleDB(a adapters.Adapter) (bool, error) {
	d, ok := a.(adapters.DatabaseAccessor)
	if !ok {
		return false, ErrNotTimescaleDBAdapter
	}
	db, err := d.DB()
	if err != nil {
		return false, err
	}
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='timescaledb')").Scan(&exists)
	return exists, err
}

// Close shuts down all pooled connections (delegates to postgres.Close).
func Close(a adapters.Adapter) {
	postgres.Close(a)
}

// DB returns the default sqlx connection (delegates to postgres.DB).
func DB(a adapters.Adapter) (*sqlx.DB, error) {
	return postgres.DB(a)
}

// Ping verifies database connectivity (delegates to postgres.Ping).
func Ping(ctx context.Context, a adapters.Adapter) error {
	return postgres.Ping(ctx, a)
}
