package logsafe

import (
	"errors"
	"regexp"
)

var (
	passwordKV = regexp.MustCompile(`(?i)password=[^\s'"]+`)
	pgURLCreds = regexp.MustCompile(`postgres(?:ql)?://([^:@/]+):([^@/]+)@`)
)

// Error returns err with database credentials redacted for safe structured logging.
func Error(err error) error {
	if err == nil {
		return nil
	}
	redacted := passwordKV.ReplaceAllString(err.Error(), "password=***")
	redacted = pgURLCreds.ReplaceAllString(redacted, "postgres://$1:***@")
	if redacted == err.Error() {
		return err
	}
	return errors.New(redacted)
}
