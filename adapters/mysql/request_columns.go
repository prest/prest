package mysql

import (
	"net/http"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/internal/reqcols"
)

// RequestColumns lists the columns a read request filters, groups, orders or
// counts by, plus its _join targets, so the controller can require read
// permission on each (GHSA-48r9-g6f9-x3gm). MySQL rejects _korder, so the
// shared positions are all there is.
func (a *Adapter) RequestColumns(r *http.Request) (adapters.RequestColumns, error) {
	q := r.URL.Query()
	var joins []string
	for _, clause := range q["_join"] {
		if target, ok := joinTargetByClause(clause, ""); ok {
			joins = append(joins, target.table)
		}
	}
	return adapters.RequestColumns{Columns: reqcols.Extract(q), JoinTables: joins}, nil
}
