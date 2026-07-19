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

		// Invalid expressions (injection attempts)
		{"time_bucket('1 minute'; DROP TABLE users; time)", false},
		{"time_bucket('1 minute'--comment", false},
		{"time_bucket(1 minute, time)", true}, // Missing quotes but syntactically ok per our check
		{"time_bucket('1 minute', time", false}, // unbalanced parentheses
		{"time_bucket)('1 minute', time)", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result := isSafeSQLExpression(tt.expr)
			require.Equal(t, tt.expected, result, "expr=%s", tt.expr)
		})
	}
}
