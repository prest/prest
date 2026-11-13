package timescaledb

import (
	"net/http"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/internal/ident"
)

// RequestColumns delegates to the embedded adapter and adds the _time_bucket
// column ("time" unless named), which TimeBucketClause groups by.
func (a *Adapter) RequestColumns(r *http.Request) (adapters.RequestColumns, error) {
	var refs adapters.RequestColumns
	if lister, ok := a.Adapter.(adapters.RequestColumnsLister); ok {
		var err error
		if refs, err = lister.RequestColumns(r); err != nil {
			return refs, err
		}
	}
	if param := strings.TrimSpace(r.URL.Query().Get("_time_bucket")); param != "" {
		col := "time"
		if parts := strings.Split(param, ","); len(parts) > 1 {
			col = strings.TrimSpace(parts[1])
		}
		if ident.IsValid(col) {
			refs.Columns = append(refs.Columns, col)
		}
	}
	return refs, nil
}
