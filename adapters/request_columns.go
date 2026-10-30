package adapters

import "net/http"

// RequestColumns is what a read request references outside _select.
type RequestColumns struct {
	// Columns are the columns filtered, grouped, ordered or counted by, as
	// the request names them: "ssn" or "employee_secret.ssn".
	Columns []string
	// JoinTables are the _join target tables, schema stripped.
	JoinTables []string
}

// RequestColumnsLister lists the columns a read request filters, groups,
// orders or counts by, so the controller can require read permission on each
// (GHSA-48r9-g6f9-x3gm). Optional: an adapter without it gets no extra check.
type RequestColumnsLister interface {
	RequestColumns(r *http.Request) (RequestColumns, error)
}
