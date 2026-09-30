package mysql

import (
	"fmt"
	"strings"

	"github.com/prest/prest/v2/internal/ident"
)

func quoteIdent(s string) (string, error) {
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		out := make([]string, len(parts))
		for i, part := range parts {
			q, err := quoteIdent(part)
			if err != nil {
				return "", err
			}
			out[i] = q
		}
		return strings.Join(out, "."), nil
	}
	if !ident.IsValid(s) && !ident.IsSafeSegment(s) {
		return "", fmt.Errorf("%w: %s", errInvalidIdentifier, s)
	}
	return "`" + strings.ReplaceAll(s, "`", "``") + "`", nil
}

func mustQuote(s string) string {
	q, err := quoteIdent(s)
	if err != nil {
		return ""
	}
	return q
}

// placeholders returns n question-mark placeholders wrapped in parentheses.
// initial is ignored for the text; it only preserves the caller's value order.
func placeholders(initial, lenValues int) string {
	n := lenValues - initial + 1
	if n < 0 {
		n = 0
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return "(" + strings.Join(parts, ",") + ")"
}
