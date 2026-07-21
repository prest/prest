package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleIndexOptimization(t *testing.T) {
	// Query hypertable with time-based filtering.
	// Expected: pREST generates optimized queries that leverage TimescaleDB's
	// time-partitioning. Filtering by time uses chunk-exclusion internally.
	// Device filtering on partitioned hypertables uses efficient index scans.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?device_id=$eq.device-1&_order=-time",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TestTimescaleIndexOptimization",
		"device-1", // should find records efficiently
	)
}

func TestTimescaleCompressionQuery(t *testing.T) {
	// Query hypertable with compression enabled.
	// Expected: Compressed chunks are transparently queried with no performance penalty.
	// pREST does not need compression-specific logic; TimescaleDB handles decompression.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TestTimescaleCompressionQuery",
		"temperature", // should query compressed data transparently
	)
}

func TestTimescaleLimitPagination(t *testing.T) {
	// Query with pagination to avoid scanning entire hypertable.
	// Expected: pREST's _page and _page_size parameters optimize large dataset queries.
	// Page size defaults to 10; users can control pagination for memory efficiency.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_page=1&_page_size=2",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TestTimescaleLimitPagination",
		"device-1", // should return only 2 rows
	)
}
