package timescaledb

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func testAdapter() adapters.Adapter {
	cfg := &config.Prest{}
	return New(cfg)
}

func TestTimeBucketClause(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()

	testCases := []struct {
		name      string
		url       string
		expected  string
		hasError  bool
		errorText string
	}{
		{
			name:     "5 minute bucket with default time column",
			url:      "/?_time_bucket=5m",
			expected: `GROUP BY time_bucket('5 minutes', "time")`,
		},
		{
			name:     "1 hour bucket with default time column",
			url:      "/?_time_bucket=1h",
			expected: `GROUP BY time_bucket('1 hour', "time")`,
		},
		{
			name:     "1 day bucket with custom column",
			url:      "/?_time_bucket=1d,created_at",
			expected: `GROUP BY time_bucket('1 day', "created_at")`,
		},
		{
			name:     "6 hour bucket",
			url:      "/?_time_bucket=6h",
			expected: `GROUP BY time_bucket('6 hours', "time")`,
		},
		{
			name:     "7 day bucket",
			url:      "/?_time_bucket=7d",
			expected: `GROUP BY time_bucket('7 days', "time")`,
		},
		{
			name:     "30 day bucket",
			url:      "/?_time_bucket=30d",
			expected: `GROUP BY time_bucket('30 days', "time")`,
		},
		{
			name:     "1 year bucket",
			url:      "/?_time_bucket=1y",
			expected: `GROUP BY time_bucket('1 year', "time")`,
		},
		{
			name:     "15 minute bucket with spaces in parameter",
			url:      "/?_time_bucket= 15m , created_at ",
			expected: `GROUP BY time_bucket('15 minutes', "created_at")`,
		},
		{
			name:      "empty _time_bucket",
			url:       "/",
			expected:  "",
			hasError:  false,
		},
		{
			name:      "invalid interval",
			url:       "/?_time_bucket=2h",
			hasError:  true,
			errorText: "invalid time_bucket interval",
		},
		{
			name:      "invalid column name",
			url:       "/?_time_bucket=1h,0invalid",
			hasError:  true,
			errorText: "invalid column name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			clause, err := adapter.TimeBucketClause(req)
			if tc.hasError {
				require.Error(t, err)
				if tc.errorText != "" {
					require.Contains(t, err.Error(), tc.errorText)
				}
				return
			}
			require.NoError(t, err)
			if tc.expected == "" {
				require.Empty(t, clause)
			} else {
				require.Equal(t, tc.expected, clause)
			}
		})
	}
}
