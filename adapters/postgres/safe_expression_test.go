package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSafeSQLExpression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expr     string
		expected bool
	}{
		// Valid expressions
		{"time_bucket('1 minute', time)", true},
		{"time_bucket('1 hour', created_at)", true},
		{"date_trunc('day', updated_at)", true},
		{"extract(hour from event_time)", true},
		{"upper(name)", true},

		// Invalid expressions (injection attempts)
		{"time_bucket('1 minute'; DROP TABLE users; time)", false},
		{"time_bucket('1 minute'--comment)", false},
		{"upper(name)--x", false},
		{"pg_sleep(1)", false},
		{"pg_read_file('/etc/passwd')", false},
		{"unknown_func(name)", false},
		{"time_bucket('1 minute', time", false},
		{"time_bucket)('1 minute', time)", false},

		// Nested subquery / trailing statement injection attempts (CWE-89 sibling
		// to the _select fix): a balanced-paren, comment-free expression that
		// smuggles a subquery or an extra statement past the outer allowlisted call.
		{"upper((SELECT 1))", false},
		{"upper((SELECT 1)) UNION SELECT current_user", false},
		{"abs((SELECT 1 FROM pg_sleep(3)))", false},
		{"upper(name) UNION SELECT current_user", false},
		{"coalesce(nullif(name, ''), 'x')", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result := isSafeSQLExpression(tt.expr)
			require.Equal(t, tt.expected, result, "expr=%s", tt.expr)
		})
	}
}
