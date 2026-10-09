package mysql

import (
	"encoding/json"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/stretchr/testify/require"
)

// Columns missing from the first record must not be dropped; absent slots
// render DEFAULT and carry a DefaultValue marker to keep values aligned.
func TestParseBatchInsertUnionOfKeys(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	names, ph, vals, err := a.ParseBatchInsertRequest(bodyReq("/t", `[{"a":1},{"b":2}]`))
	require.NoError(t, err)
	require.Equal(t, "`a`,`b`", names)
	require.Equal(t, "(?,DEFAULT),(DEFAULT,?)", ph)
	require.Equal(t, []interface{}{json.Number("1"), adapters.DefaultValue{}, adapters.DefaultValue{}, json.Number("2")}, vals)
}

// Uniform records keep the plain placeholder shape with no markers.
func TestParseBatchInsertUniformRecords(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	names, ph, vals, err := a.ParseBatchInsertRequest(bodyReq("/t", `[{"b":"z","a":1},{"a":2,"b":"y"}]`))
	require.NoError(t, err)
	require.Equal(t, "`a`,`b`", names)
	require.Equal(t, "(?,?),(?,?)", ph)
	require.Equal(t, []interface{}{json.Number("1"), "z", json.Number("2"), "y"}, vals)
	require.False(t, adapters.HasDefaults(vals))
}

// Every record's keys are validated, not only the first record's.
func TestParseBatchInsertInvalidKeyInLaterRecord(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	_, _, _, err := a.ParseBatchInsertRequest(bodyReq("/t", `[{"a":1},{"bad name":2}]`))
	require.ErrorIs(t, err, errInvalidIdentifier)
}

// Integers above 2^53 must keep every digit through decode and bind.
func TestBodyDecodingKeepsLargeIntegers(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)

	_, _, vals, err := a.ParseInsertRequest(bodyReq("/t", `{"n":9007199254740993}`))
	require.NoError(t, err)
	require.Equal(t, []interface{}{json.Number("9007199254740993")}, vals)

	_, _, vals, err = a.ParseInsertRequest(bodyReq("/t", `{"o":{"n":9007199254740993}}`))
	require.NoError(t, err)
	require.Equal(t, []interface{}{`{"n":9007199254740993}`}, vals)

	_, vals, err = a.SetByRequest(bodyReq("/t", `{"n":18446744073709551615}`), 1)
	require.NoError(t, err)
	require.Equal(t, []interface{}{json.Number("18446744073709551615")}, vals)

	_, _, vals, err = a.ParseBatchInsertRequest(bodyReq("/t", `[{"n":9007199254740993}]`))
	require.NoError(t, err)
	require.Equal(t, []interface{}{json.Number("9007199254740993")}, vals)
}
