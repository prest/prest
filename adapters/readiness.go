package adapters

import "context"

// DatabasePinger checks whether the default database connection is alive.
type DatabasePinger interface {
	Ping(ctx context.Context) error
}

// ReadinessChecker verifies all registered database connections are reachable.
type ReadinessChecker interface {
	PingAll(ctx context.Context) error
}
