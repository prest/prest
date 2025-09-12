package ident

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var re = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// IsValid reports whether s is a valid SQL identifier or dotted identifier path.
func IsValid(s string) bool {
	if !re.MatchString(s) {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if len(part) == 0 || len(part) > 63 {
			return false
		}
	}
	return true
}

// IsSafeSegment reports whether s is a safe, single identifier segment for path params
// like database, schema, or table. It allows letters, digits, underscore and hyphen,
// PostgreSQL identifiers don't support. These must be quoted when used in SQL.
// with length up to 63, and disallows dots and quotes.
func IsSafeSegment(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// Quote validates and returns a safely quoted identifier path like "a"."b".
func Quote(s string) (string, error) {
	if !IsValid(s) {
		return "", fmt.Errorf("invalid identifier: %s", s)
	}
	parts := strings.Split(s, ".")
	for i := range parts {
		parts[i] = `"` + strings.ReplaceAll(parts[i], `"`, `""`) + `"`
	}
	return strings.Join(parts, "."), nil
}

// SplitAndValidateCSV splits a comma-separated list and validates each identifier.
func SplitAndValidateCSV(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	for _, p := range parts {
		if !IsValid(p) {
			return nil, fmt.Errorf("invalid identifier: %s", p)
		}
	}
	return parts, nil
}
