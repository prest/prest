package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleHypertablePermissions(t *testing.T) {
	// Query a hypertable in TimescaleDB.
	// Expected: Hypertables inherit table permissions from the database ACL system.
	// Since sensor_data is a hypertable in the public schema, queries use standard
	// table permissions. No separate TimescaleDB-specific ACL is needed.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TestTimescaleHypertablePermissions",
		"device_id", // table should be queryable
	)
}

func TestTimescaleContinuousAggregatePermissions(t *testing.T) {
	// Query a continuous aggregate (materialized view) in TimescaleDB.
	// Expected: Continuous aggregates are materialized views and inherit view permissions.
	// sensor_data_hourly is a materialized view created by TimescaleDB.
	// Note: Response may be empty array if time_bucket results are empty,
	// but the endpoint should return 200 OK.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data_hourly",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TestTimescaleContinuousAggregatePermissions",
	)
}
