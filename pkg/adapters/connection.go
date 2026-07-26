package adapters

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// DatabaseConnector initializes the adapter connection pool.
//
// Optional capability: not embedded in Adapter. Implementations such as the
// postgres adapter also satisfy this interface; callers use type assertions.
type DatabaseConnector interface {
	Connect() error
}

// DatabaseAccessor returns the default database connection.
//
// Optional capability: not embedded in Adapter. See DatabaseConnector.
type DatabaseAccessor interface {
	DB() (*sqlx.DB, error)
}

// DatabasePinger verifies database connectivity.
//
// Optional capability: not embedded in Adapter. See DatabaseConnector.
type DatabasePinger interface {
	Ping(ctx context.Context) error
}
