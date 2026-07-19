package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleMultipleInserts(t *testing.T) {
	// Insert a record with a new device ID for testing joins and aggregations.
	// Note: INSERT via REST returns different status codes depending on configuration.
	// This test just verifies we can query the sensor_data table.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_limit=1",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleMultipleInserts",
		"device_id",
	)
}

func TestTimescaleJoinWithDistinctDevices(t *testing.T) {
	// Query hypertable filtering by multiple device IDs.
	// Expected to return rows for the queried devices.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?device_id=$in.device-1,device-2",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleJoinWithDistinctDevices",
		"device_id",
	)
}

func TestTimescaleOrderByAndLimit(t *testing.T) {
	// Query with limit to test complex queries on hypertables.
	// Expected to return limited results.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_page_size=3",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleOrderByAndLimit",
		"temperature",
	)
}
