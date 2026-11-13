package timescaledb

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/stretchr/testify/require"
)

var _ adapters.RequestColumnsLister = (*Adapter)(nil)

// TestRequestColumns_TimeBucket adds the _time_bucket column, "time" by
// default, to the Postgres columns: it is grouped by, so it must be readable.
func TestRequestColumns_TimeBucket(t *testing.T) {
	t.Parallel()

	lister := testAdapter().(adapters.RequestColumnsLister)
	cases := []struct {
		query string
		want  []string
	}{
		{"?_time_bucket=1h,created_at&dept=$eq.x", []string{"dept", "created_at"}},
		{"?_time_bucket=1h", []string{"time"}},
		{"?_time_bucket=1h,bad-col", nil},
		{"?dept=$eq.x", []string{"dept"}},
	}
	for _, tc := range cases {
		got, err := lister.RequestColumns(httptest.NewRequest(http.MethodGet, "/db/public/t"+tc.query, nil))
		require.NoError(t, err)
		require.ElementsMatch(t, tc.want, got.Columns, tc.query)
	}
}

// An embedded adapter without the lister yields no columns: no extra check.
func TestRequestColumns_EmbeddedWithoutLister(t *testing.T) {
	t.Parallel()

	a := &Adapter{}
	got, err := a.RequestColumns(httptest.NewRequest(http.MethodGet, "/db/public/t?_time_bucket=1h", nil))
	require.NoError(t, err)
	require.Equal(t, []string{"time"}, got.Columns)
}
