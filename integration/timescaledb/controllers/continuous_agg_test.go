package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleContinuousAggregateDiscovery(t *testing.T) {
	// Verify that continuous aggregates (materialized views) are discoverable in schema.
	// Expected to succeed with HTTP status OK and include sensor_data hypertable name.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public?t.tablename=$like.sensor%",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleContinuousAggregateDiscovery",
		"sensor_data",
	)
}
