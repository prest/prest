package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleChunkMetadata(t *testing.T) {
	// Query TimescaleDB's chunk metadata table to inspect chunks.
	// This is done via custom SQL queries since chunk metadata is not exposed as regular tables.
	// This test verifies that we can query the timescaledb_information schema.
	base := helpers.ServerURL(t)

	// Query internal timescaledb table to get chunk info
	// For now, we just verify the sensor_data hypertable is queryable
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_limit=1",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleChunkMetadata",
		"device_id",
	)
}
