package mysql

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBytesBinaryKindsAsHex(t *testing.T) {
	t.Parallel()
	raw := []byte{0xde, 0xad, 0xbe, 0xef, 0x00}
	for _, kind := range []string{"BLOB", "BINARY", "VARBINARY", "BIT", "GEOMETRY"} {
		require.Equal(t, `\xdeadbeef00`, normalizeBytes(raw, kind), kind)
	}
}

func TestNormalizeBytesTextKindsStayStrings(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"TEXT", "VARCHAR", "CHAR", ""} {
		require.Equal(t, "héllo", normalizeBytes([]byte("héllo"), kind), kind)
	}
	require.Equal(t, "9.50", normalizeBytes([]byte("9.50"), "DECIMAL"))
	require.Equal(t, json.RawMessage(`{"a":1}`), normalizeBytes([]byte(`{"a":1}`), "JSON"))
}

func TestNormalizeBytesIntegerKindsAsNumber(t *testing.T) {
	t.Parallel()
	got := normalizeBytes([]byte("18446744073709551615"), "UNSIGNED BIGINT")
	require.Equal(t, json.Number("18446744073709551615"), got)
	out, err := json.Marshal(map[string]any{"n": got})
	require.NoError(t, err)
	require.JSONEq(t, `{"n":18446744073709551615}`, string(out))
	require.Equal(t, `{"n":18446744073709551615}`, string(out))

	for _, kind := range []string{"TINYINT", "SMALLINT", "MEDIUMINT", "INT", "BIGINT", "UNSIGNED INT"} {
		require.Equal(t, json.Number("-42"), normalizeBytes([]byte("-42"), kind), kind)
	}
	// Non-digit payloads are not trusted as numbers.
	require.Equal(t, "12a", normalizeBytes([]byte("12a"), "INT"))
	require.Equal(t, "-", normalizeBytes([]byte("-"), "INT"))
	require.Equal(t, "", normalizeBytes([]byte(""), "INT"))
}

func TestNormalizeScannedTimeByKind(t *testing.T) {
	t.Parallel()
	whole := time.Date(2024, 3, 5, 14, 7, 9, 0, time.UTC)
	frac := time.Date(2024, 3, 5, 14, 7, 9, 123000000, time.UTC)

	require.Equal(t, "2024-03-05", normalizeScanned(whole, "DATE"))
	require.Equal(t, "2024-03-05T14:07:09", normalizeScanned(whole, "DATETIME"))
	require.Equal(t, "2024-03-05T14:07:09.123", normalizeScanned(frac, "DATETIME"))
	require.Equal(t, "2024-03-05T14:07:09+00:00", normalizeScanned(whole, "TIMESTAMP"))
	require.Equal(t, "2024-03-05T14:07:09.123+00:00", normalizeScanned(frac, "timestamp"))
	require.Equal(t, frac.Format(time.RFC3339Nano), normalizeScanned(frac, ""))

	// TIMESTAMP values are normalized to UTC before formatting.
	est := time.FixedZone("EST", -5*3600)
	require.Equal(t, "2024-03-05T19:07:09+00:00", normalizeScanned(whole.In(est).Add(5*time.Hour), "TIMESTAMP"))
}
