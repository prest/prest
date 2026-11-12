package controllers_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

const sessionTimezonePath = "/_QUERIES/timezone/session"

// getSessionTimezone runs the session probe script once on base and returns
// the session TimeZone and the fixed instant 2024-01-01T12:00:00Z as rendered
// in that zone; it fails the test unless the response is 200 with one row.
func getSessionTimezone(t *testing.T, base string) (string, string) {
	t.Helper()
	var rows []struct {
		Timezone string `json:"timezone"`
		TS       string `json:"ts"`
	}
	testutils.DoRequestJSON(t, base+sessionTimezonePath, nil, "GET", http.StatusOK, "ExecuteScriptQuery", &rows)
	require.Len(t, rows, 1)
	return rows[0].Timezone, rows[0].TS
}

// Server started with PREST_PG_TIMEZONE=America/Sao_Paulo: the session zone
// and the rendered timestamptz offset both follow it.
func TestSessionTimezone_ConfiguredZone(t *testing.T) {
	tz, ts := getSessionTimezone(t, helpers.TimezoneServerURL(t))
	require.Equal(t, "America/Sao_Paulo", tz)
	require.Equal(t, "2024-01-01T09:00:00-03:00", ts)
}

// Server without PREST_PG_TIMEZONE keeps the server default: not Sao Paulo, no -03:00 offset.
func TestSessionTimezone_DefaultUnchanged(t *testing.T) {
	tz, ts := getSessionTimezone(t, helpers.ServerURL(t))
	require.NotEqual(t, "America/Sao_Paulo", tz)
	require.False(t, strings.HasSuffix(ts, "-03:00"), ts)
}
