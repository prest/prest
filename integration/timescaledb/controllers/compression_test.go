package timescaledb_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
)

func TestTimescaleCompressionSetup(t *testing.T) {
	// Verify that we can set up compression on a hypertable.
	// In a real scenario, this would be done via /_QUERIES custom SQL.
	// This test just verifies the hypertable is queryable after setup.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_page_size=1",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleCompressionSetup",
		"device_id",
	)
}

func TestTimescaleRetentionPolicy(t *testing.T) {
	// Verify hypertable remains queryable with retention policies configured.
	// Retention policies are managed via database SQL, not REST.
	// This test verifies the table is queryable.
	base := helpers.ServerURL(t)
	testutils.DoRequest(
		t,
		base+"/prest-test/public/sensor_data?_page_size=2&_order=-time",
		nil,
		http.MethodGet,
		http.StatusOK,
		"TimescaleRetentionPolicy",
		"temperature",
	)
}
