package adapters

import (
	"database/sql/driver"
	"errors"
	"net"
	"syscall"
)

// ErrRelationNotFound is returned when a statement targets a table or view
// that does not exist. MySQL error 1146 is wrapped with this sentinel.
// Postgres keeps its driver string ("pq: relation ... does not exist"); the
// CRUD handler treats both as HTTP 404.
var ErrRelationNotFound = errors.New("relation not found")

// ErrUnavailable marks a failure to reach the database (refused, reset, bad
// connection). Handlers answer 503 without the driver text.
var ErrUnavailable = errors.New("database unavailable")

// IsUnavailable reports connection-level failures from any engine: the
// sentinel, stdlib network errors, and driver.ErrBadConn.
func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrUnavailable) || errors.Is(err, driver.ErrBadConn) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var op *net.OpError
	return errors.As(err, &op)
}
