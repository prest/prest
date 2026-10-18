package access

import (
	"errors"
	"strings"

	"github.com/prest/prest/v2/config"
)

const (
	defaultQueriesTable = "prest_queries"
	defaultAuthTable    = "prest_users"
)

// ErrReservedTable is returned when a data-plane request names one of pREST's
// own control-plane tables, directly or through a _join.
var ErrReservedTable = errors.New("table is reserved for pREST internal use")

// ReservedTable reports whether table is one of pREST's own control-plane
// tables (stored queries, auth users). Matching is by name only, case-insensitive,
// in any schema: an unqualified _join target resolves through search_path, and
// MCP checks run against the default adapter's config, so a schema-qualified match
// could be sidestepped. These tables are never reachable from data-plane routes,
// whatever access.restrict or ignore_table say (GHSA-2x9q-r7j9-g85f).
func ReservedTable(cfg *config.Prest, table string) bool {
	if table == "" {
		return false
	}
	names := []string{defaultQueriesTable, defaultAuthTable}
	if cfg != nil {
		names = append(names, cfg.QueriesConf.Table, cfg.AuthTable)
	}
	for _, name := range names {
		if name != "" && strings.EqualFold(name, table) {
			return true
		}
	}
	return false
}
