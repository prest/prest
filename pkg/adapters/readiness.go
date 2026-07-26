package adapters

import "context"

// ReadinessChecker verifies all registered database connections are reachable.
type ReadinessChecker interface {
	PingAll(ctx context.Context) error
}
