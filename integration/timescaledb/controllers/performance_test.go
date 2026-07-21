package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleTimeSeriesQuery(t *testing.T) {
	// Query sensor data with time-based filtering.
	// Expected to efficiently return filtered time-series data.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?time=$gte.2026-07-19T00:00:00Z",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleTimeSeriesQuery",
		"temperature",
	)
}

func TestTimescaleLargePageSize(t *testing.T) {
	// Test querying with large page size to verify handling of multiple rows.
	// Expected to return all available data without error.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_page_size=1000",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleLargePageSize",
		"device_id",
	)
}

func TestTimescaleFilteredTimeSeriesQuery(t *testing.T) {
	// Query with filters on device ID and temperature range.
	// Expected to efficiently return filtered subset.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?device_id=$eq.device-1",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleFilteredTimeSeriesQuery",
		"device-1",
	)
}
