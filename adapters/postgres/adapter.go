package postgres

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
)

// ErrNotPostgresAdapter is returned when an adapter does not support postgres connection helpers.
var ErrNotPostgresAdapter = errors.New("adapter is not postgres")

// Connect initializes the postgres adapter connection pool.
func Connect(a adapters.Adapter) error {
	c, ok := a.(adapters.DatabaseConnector)
	if !ok {
		return ErrNotPostgresAdapter
	}
	return c.Connect()
}

// DB returns the default sqlx connection from a postgres adapter.
func DB(a adapters.Adapter) (*sqlx.DB, error) {
	d, ok := a.(adapters.DatabaseAccessor)
	if !ok {
		return nil, ErrNotPostgresAdapter
	}
	return d.DB()
}

// Ping verifies database connectivity for a postgres adapter.
func Ping(ctx context.Context, a adapters.Adapter) error {
	p, ok := a.(adapters.DatabasePinger)
	if !ok {
		return ErrNotPostgresAdapter
	}
	return p.Ping(ctx)
}
