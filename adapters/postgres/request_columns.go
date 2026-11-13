package postgres

import (
	"net/http"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/internal/ident"
	"github.com/prest/prest/v2/internal/reqcols"
)

// RequestColumns lists the columns a read request filters, groups, orders or
// counts by, plus its _join targets, so the controller can require read
// permission on each (GHSA-48r9-g6f9-x3gm).
func (adapter *postgres) RequestColumns(r *http.Request) (adapters.RequestColumns, error) {
	q := r.URL.Query()
	cols := reqcols.Extract(q)
	// _korder's column, as buildVectorOrderTerm reads it.
	if col, _, ok := strings.Cut(q.Get("_korder"), ":"); ok && ident.IsValid(col) {
		cols = append(cols, col)
	}
	var joins []string
	for _, clause := range q["_join"] {
		if target, ok := joinTargetByClause(clause, ""); ok {
			joins = append(joins, target.table)
		}
	}
	return adapters.RequestColumns{Columns: cols, JoinTables: joins}, nil
}
