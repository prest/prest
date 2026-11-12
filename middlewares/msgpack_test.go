package middlewares

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

// float64Hex is the hex of a msgpack float64: the 0xcb marker and the
// big-endian IEEE 754 bits.
func float64Hex(f float64) string {
	return "cb" + hex.EncodeToString(binary.BigEndian.AppendUint64(nil, math.Float64bits(f)))
}

// TestRenderMsgpack_Bytes: the exact encoding of each kind of JSON value.
func TestRenderMsgpack_Bytes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc string
		body string
		want string // hex
	}{
		{
			desc: "rows become maps inside an array",
			body: `[{"name":"a","age":1}]`,
			want: "91" + "82" + "a46e616d65" + "a161" + "a3616765" + "01",
		},
		{
			desc: "keys keep the order of the body and a repeated key keeps both values",
			body: `{"b":1,"a":2,"b":3}`,
			want: "83" + "a162" + "01" + "a161" + "02" + "a162" + "03",
		},
		{
			desc: "empty containers",
			body: ` {"a":[],"b":{}} `,
			want: "82" + "a161" + "90" + "a162" + "80",
		},
		{
			desc: "strings are unescaped",
			body: `"é<&>"`,
			want: "a5" + "c3a9" + "3c263e",
		},
		{
			desc: "literals",
			body: `[null,true,false]`,
			want: "93" + "c0" + "c3" + "c2",
		},
		{
			desc: "integers use the smallest encoding",
			body: `[0,-0,127,128,-1,-32,-33,65536,-2147483649]`,
			want: "99" + "00" + "00" + "7f" + "cc80" + "ff" + "e0" + "d0df" + "ce00010000" + "d3ffffffff7fffffff",
		},
		{
			desc: "integers at the limits of 64 bits",
			body: `[9223372036854775807,18446744073709551615,-9223372036854775808]`,
			want: "93" + "cf7fffffffffffffff" + "cfffffffffffffffff" + "d38000000000000000",
		},
		{
			// Not an integer that fits in 64 bits: a float64, which is how
			// JSON decoders read these numbers too.
			desc: "other numbers are float64",
			body: `[1.50,-2.25e-3,1e3,18446744073709551616]`,
			want: "94" + float64Hex(1.5) + float64Hex(-0.00225) + float64Hex(1000) + float64Hex(18446744073709551616),
		},
		{
			desc: "16 elements need an array16 header",
			body: "[" + strings.TrimSuffix(strings.Repeat("1,", 16), ",") + "]",
			want: "dc0010" + strings.Repeat("01", 16),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			packed, ok := renderMsgpack([]byte(tc.body))
			require.True(t, ok)
			require.Equal(t, tc.want, hex.EncodeToString(packed))
		})
	}
}

// TestRenderMsgpack_RoundTrip: decoding the msgpack gives back the values of
// the JSON body.
func TestRenderMsgpack_RoundTrip(t *testing.T) {
	t.Parallel()

	body := `[{"id":1,"name":"prest","price":9.99,"active":true,"deleted_at":null,` +
		`"tags":["a","b"],"data":{"nested":{"n":-7}}}]`
	packed, ok := renderMsgpack([]byte(body))
	require.True(t, ok)

	var decoded any
	require.NoError(t, msgpack.Unmarshal(packed, &decoded))
	roundTrip, err := json.Marshal(decoded)
	require.NoError(t, err)
	require.JSONEq(t, body, string(roundTrip))
}

// TestRenderMsgpack_Nesting: a body may nest as deeply as encoding/json
// allows, and no deeper.
func TestRenderMsgpack_Nesting(t *testing.T) {
	t.Parallel()

	nested := func(depth int) []byte {
		return []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))
	}

	_, ok := renderMsgpack(nested(maxMsgpackDepth))
	require.True(t, ok, "as deep as encoding/json accepts")

	_, ok = renderMsgpack(nested(maxMsgpackDepth + 1))
	require.False(t, ok, "deeper than encoding/json accepts")
}

// TestRenderMsgpack_NotEncodable: a body with no msgpack form is reported,
// so the caller sends it unchanged.
func TestRenderMsgpack_NotEncodable(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc string
		body string
	}{
		{"empty body", ""},
		{"plain text", "not json"},
		{"truncated JSON", `[{"a":1}`},
		{"two JSON values", `{"a":1} {"b":2}`},
		{"a number out of the range of a float64", `[1e400]`},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			packed, ok := renderMsgpack([]byte(tc.body))
			require.False(t, ok)
			require.Nil(t, packed)
		})
	}
}
