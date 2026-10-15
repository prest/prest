package postgres

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// servedDatabases lists the databases reachable without a registry:
// pg.database plus pg.allowed_databases, empties and duplicates dropped.
// nil in registry mode, where the aliases are the source of truth.
func (p *postgres) servedDatabases() []string {
	if p.cfg.HasDatabaseRegistry() {
		return nil
	}
	return dedupeNonEmpty(append([]string{p.cfg.PGDatabase}, p.cfg.AllowedDatabases...))
}

// servedFilter restricts field to names. Names come from config, not the
// request, and are inlined with pq.QuoteLiteral so the request filter's
// placeholders ($1…) keep their numbering. No names yields FALSE.
func servedFilter(field string, names []string) string {
	if len(names) == 0 {
		return "FALSE"
	}
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = pq.QuoteLiteral(n)
	}
	return fmt.Sprintf("%s IN (%s)", field, strings.Join(quoted, ", "))
}

// dedupeNonEmpty returns names without empty strings and duplicates, keeping
// the first occurrence in order. The result is never nil.
func dedupeNonEmpty(names []string) []string {
	out := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}
