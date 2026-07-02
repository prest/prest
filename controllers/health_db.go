package controllers

import (
	"context"
)

// DefaultCheckList is used when the adapter does not implement DatabasePinger.
var DefaultCheckList = CheckList{}

// CheckDBHealth verifies the database connection is alive.
func CheckDBHealth(ping func(context.Context) error) HealthCheckFunc {
	return ping
}
