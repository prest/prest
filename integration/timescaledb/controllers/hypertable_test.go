package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleHypertableListed(t *testing.T) {
	// List tables filtered to the seeded Timescale hypertable.
	// Expected to succeed with HTTP status OK and include sensor_data.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public?t.tablename=$eq.sensor_data",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleHypertableListed",
		"sensor_data",
	)
}

func TestTimescaleHypertableCRUD(t *testing.T) {
	// Read rows from the seeded hypertable through the table GET endpoint.
	// Expected to succeed with HTTP status OK and return inserted sensor rows.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_page_size=5",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleHypertableRead",
		"device-1",
		"temperature",
	)
}

func TestTimescaleTimeBucketQuery(t *testing.T) {
	// Query with time_bucket operator to aggregate rows by time intervals.
	// Expected to succeed with HTTP status OK and return bucketed time data.
	// The response should contain time_bucket column with bucketed timestamps.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_groupby=time_bucket(%271%20minute%27,time)",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleTimeBucketQuery",
		"time",
	)
}
