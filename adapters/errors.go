package adapters

import "errors"

// ErrRelationNotFound is returned when a statement targets a table or view
// that does not exist. MySQL error 1146 is wrapped with this sentinel.
// Postgres keeps its driver string ("pq: relation ... does not exist"); the
// CRUD handler treats both as HTTP 404.
var ErrRelationNotFound = errors.New("relation not found")
