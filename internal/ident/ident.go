package ident

import (
	"fmt"
	"regexp"
	"strings"
)

var re = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// IsValid reports whether s is a valid SQL identifier or dotted identifier path.
func IsValid(s string) bool {
	return re.MatchString(s)
}

// Quote validates and returns a safely quoted identifier path like "a"."b".
func Quote(s string) (string, error) {
	if !IsValid(s) {
		return "", fmt.Errorf("invalid identifier: %s", s)
	}
	parts := strings.Split(s, ".")
	for i := range parts {
		parts[i] = `"` + parts[i] + `"`
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
