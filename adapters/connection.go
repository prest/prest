package adapters

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// DatabaseConnector initializes the adapter connection pool.
type DatabaseConnector interface {
	Connect() error
}

// DatabaseAccessor returns the default database connection.
type DatabaseAccessor interface {
	DB() (*sqlx.DB, error)
}

// DatabasePinger verifies database connectivity.
type DatabasePinger interface {
	Ping(ctx context.Context) error
}
