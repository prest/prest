package logsafe

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError_nil(t *testing.T) {
	t.Parallel()

	require.Nil(t, Error(nil))
}

func TestError_passwordKV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain",
			input: `connect failed: user=u password=secret dbname=db host=localhost`,
			want:  `connect failed: user=u password=*** dbname=db host=localhost`,
		},
		{
			name:  "single-quoted",
			input: `connect failed: password='s3cret!' dbname=db`,
			want:  `connect failed: password=*** dbname=db`,
		},
		{
			name:  "double-quoted",
			input: `connect failed: password="s3cret!" dbname=db`,
			want:  `connect failed: password=*** dbname=db`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.input)
			redacted := Error(err)
			require.Equal(t, tt.want, redacted.Error())
		})
	}
}

func TestError_mysqlURL(t *testing.T) {
	t.Parallel()

	_, err := url.Parse("mysql://app:s3cret@db.internal:abc/shop")
	require.Error(t, err)
	require.Contains(t, err.Error(), "s3cret")

	redacted := Error(err)
	require.NotContains(t, redacted.Error(), "s3cret")
	require.Equal(t, `parse "mysql://app:***@db.internal:abc/shop": invalid port ":abc" after host`, redacted.Error())
}

func TestError_mysqlPasswordContainingAt(t *testing.T) {
	t.Parallel()

	err := errors.New("dial: mysql://user:p@ss@word@host/db failed")
	redacted := Error(err)
	require.Equal(t, "dial: mysql://user:***@host/db failed", redacted.Error())
	require.NotContains(t, redacted.Error(), "ss@word")
}

func TestRedact_mysqlUppercaseScheme(t *testing.T) {
	t.Parallel()

	redacted := Redact("MYSQL://app:s3cret@db.internal/shop")
	require.NotContains(t, redacted, "s3cret")
	require.Equal(t, "mysql://app:***@db.internal/shop", redacted)
}

func TestError_postgresURL(t *testing.T) {
	t.Parallel()

	err := errors.New(`parse "postgresql://admin:supersecret@db.example.com:5432/app": invalid port`)
	redacted := Error(err)
	require.Equal(t, `parse "postgres://admin:***@db.example.com:5432/app": invalid port`, redacted.Error())
}

func TestError_unchanged(t *testing.T) {
	t.Parallel()

	err := errors.New("connection refused")
	require.Same(t, err, Error(err))
}

// TestError_passwordContainingAt guards against the greedy-vs-first-match
// regex bug: a password containing "@" must be fully redacted, not just up
// to its first "@" (which used to leave the tail of the real password, e.g.
// "ss@word", in the clear).
func TestError_passwordContainingAt(t *testing.T) {
	t.Parallel()

	err := errors.New("dial: postgres://user:p@ss@word@host/db failed")
	redacted := Error(err)
	require.Equal(t, "dial: postgres://user:***@host/db failed", redacted.Error())
	require.NotContains(t, redacted.Error(), "ss@word")
}

func TestRedact_plainURL(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"postgres://admin:***@db.example.com:5432/app",
		Redact("postgresql://admin:supersecret@db.example.com:5432/app"),
	)
}

func TestRedact_unchanged(t *testing.T) {
	t.Parallel()

	require.Equal(t, "no credentials here", Redact("no credentials here"))
}

// TestRedact_uppercaseScheme guards against a case-sensitive scheme match:
// URL schemes are case-insensitive, so "POSTGRES://" must be redacted the
// same as "postgres://" instead of leaking the password unredacted.
func TestRedact_uppercaseScheme(t *testing.T) {
	t.Parallel()

	redacted := Redact("POSTGRES://admin:supersecret@db.example.com:5432/app")
	require.NotContains(t, redacted, "supersecret")
}
