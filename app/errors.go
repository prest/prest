package app

import "errors"

// ErrAdapterNotPostgres is returned when the configured adapter is not postgres.
var ErrAdapterNotPostgres = errors.New("adapter is not postgres")

// ErrAdapterNotQueryRegistry is returned when queries.import_on_startup requires QueryRegistry.
var ErrAdapterNotQueryRegistry = errors.New("adapter does not implement QueryRegistry")
