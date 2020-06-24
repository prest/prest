package dbtime

import (
	"fmt"
	"time"
)

// Time replace MarshalJSON and UnmarshalJSON functions to allow
// compatibility with the Postgresql date format.
type Time struct {
	time.Time
}

const layout = "2006-01-02T15:04:05.999999"

// UnmarshalJSON compatibility with the Postgresql date format
func (t *Time) UnmarshalJSON(b []byte) (err error) {
	if b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}
	if string(b) == `null` {
		*t = Time{}
		return
	}
	t.Time, err = time.Parse(layout, string(b))
	return
}

// MarshalJSON compatibility with the Postgresql date format
func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, t.Time.Format(layout))), nil
}
