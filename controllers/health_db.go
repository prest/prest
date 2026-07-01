package controllers

import (
	"context"
)

// DefaultCheckList is empty; health checks are wired in app.New.
var DefaultCheckList = CheckList{}

// CheckDBHealth verifies the database connection is alive.
func CheckDBHealth(ping func(context.Context) error) HealthCheckFunc {
	return ping
}
