package logsafe

import (
	"errors"
	"regexp"
)

var (
	passwordKV = regexp.MustCompile(`(?i)password=(?:'[^']*'|"[^"]*"|[^\s'"]+)`)
	// The password group is greedy so it consumes up to the LAST "@" before
	// the host rather than the first: a password containing "@" (e.g.
	// postgres://user:p@ss@word@host/db) would otherwise leave everything
	// after its first "@" ("ss@word") unredacted.
	pgURLCreds = regexp.MustCompile(`(?i)postgres(?:ql)?://([^:@/]+):(.+)@`)
)

// Redact returns s with database credentials removed, suitable for safe
// structured logging — e.g. a raw connection URL logged as a slog field
// value, not just an error message.
func Redact(s string) string {
	redacted := passwordKV.ReplaceAllString(s, "password=***")
	return pgURLCreds.ReplaceAllString(redacted, "postgres://$1:***@")
}

// Error returns err with database credentials redacted for safe structured logging.
func Error(err error) error {
	if err == nil {
		return nil
	}
	redacted := Redact(err.Error())
	if redacted == err.Error() {
		return err
	}
	return errors.New(redacted)
}
