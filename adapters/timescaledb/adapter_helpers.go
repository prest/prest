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
