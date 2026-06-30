package controllers

import (
	"context"

	"github.com/prest/prest/v2/adapters/postgres"
)

// DefaultCheckList is the default set of health checks.
var DefaultCheckList = CheckList{
	CheckDBHealth,
}

// CheckDBHealth verifies the database connection is alive.
func CheckDBHealth(ctx context.Context) error {
	conn, err := postgres.Get()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, ";")
	return err
}
