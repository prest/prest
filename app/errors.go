package app

import "errors"

// ErrAdapterNotPostgres is returned when the configured adapter is not postgres.
var ErrAdapterNotPostgres = errors.New("adapter is not postgres")
