package postgres

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 2^53+1 cannot be represented as float64; it must reach the driver unchanged.
const bigInt = "9007199254740993"

func numberRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	require.NoError(t, err)
	return req
}

// Insert keeps big integers exact and renders numeric arrays as {1,2}.
func TestParseInsertRequest_PreservesBigIntegers(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	_, _, values, err := adapter.ParseInsertRequest(numberRequest(t, `{"id":`+bigInt+`}`))
	require.NoError(t, err)
	require.Equal(t, []interface{}{json.Number(bigInt)}, values)

	_, _, values, err = adapter.ParseInsertRequest(numberRequest(t, `{"ids":[1,2]}`))
	require.NoError(t, err)
	require.Equal(t, []interface{}{"{1,2}"}, values)
}

// SET keeps big integers exact, including inside nested JSON objects.
func TestSetByRequest_PreservesBigIntegers(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	_, values, err := adapter.SetByRequest(numberRequest(t, `{"id":`+bigInt+`}`), 1)
	require.NoError(t, err)
	require.Equal(t, []interface{}{json.Number(bigInt)}, values)

	_, values, err = adapter.SetByRequest(numberRequest(t, `{"doc":{"n":`+bigInt+`}}`), 1)
	require.NoError(t, err)
	require.Equal(t, []interface{}{`{"n":` + bigInt + `}`}, values)
}

// Batch insert keeps big integers exact and numeric arrays intact.
func TestParseBatchInsertRequest_PreservesBigIntegers(t *testing.T) {
	t.Parallel()

	adapter := testAdapter()
	_, _, values, err := adapter.ParseBatchInsertRequest(
		numberRequest(t, `[{"id":`+bigInt+`,"ids":[1,2]}]`))
	require.NoError(t, err)
	require.Equal(t, []interface{}{json.Number(bigInt), "{1,2}"}, values)
}
