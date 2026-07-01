package controllers

import (
	"context"

	"github.com/prest/prest/v2/adapters"
)

// DefaultCheckList returns the default liveness checks for /_health.
func DefaultCheckList(pinger adapters.DatabasePinger) CheckList {
	return CheckList{
		CheckDBHealth(pinger),
	}
}

// CheckDBHealth verifies the default database connection is alive.
func CheckDBHealth(pinger adapters.DatabasePinger) HealthCheckFunc {
	return func(ctx context.Context) error {
		return pinger.Ping(ctx)
	}
}

// DefaultReadyCheckList returns readiness checks for /_ready.
func DefaultReadyCheckList(checker adapters.ReadinessChecker) CheckList {
	return CheckList{
		func(ctx context.Context) error {
			return checker.PingAll(ctx)
		},
	}
}
