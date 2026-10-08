package postgres

import (
	"fmt"
	"strings"

	"github.com/prest/prest/v2/internal/ident"
)

// QuoteIdentifier implements adapters.Dialect. Each dotted part must be a valid
// identifier or safe path segment and is wrapped in double quotes.
func (adapter *postgres) QuoteIdentifier(name string) (string, error) {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		if !ident.IsValid(part) && !ident.IsSafeSegment(part) {
			return "", fmt.Errorf("invalid identifier: %s", name)
		}
		parts[i] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
	}
	return strings.Join(parts, "."), nil
}

// Placeholder implements adapters.Dialect with $n binds.
func (adapter *postgres) Placeholder(position int) string {
	return fmt.Sprintf("$%d", position)
}
