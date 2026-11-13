package middlewares

import (
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderCSV(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc string
		body string
		want string
	}{
		{
			desc: "array of rows keeps the column order of the query",
			body: `[{"name":"joao","age":20,"id":1},{"name":"maria","age":19,"id":2}]`,
			want: "name,age,id\njoao,20,1\nmaria,19,2\n",
		},
		{
			desc: "strings are unescaped and quoted only when they need it",
			body: `[{"s":"a, b"},{"s":"say \"hi\""},{"s":"line\nbreak"},{"s":"cr\r\nlf"},{"s":"\u00e9 <b>&</b>"},{"s":" padded "}]`,
			want: "s\n\"a, b\"\n\"say \"\"hi\"\"\"\n\"line\nbreak\"\n\"cr\r\nlf\"\n\u00e9 <b>&</b>\n\" padded \"\n",
		},
		{
			desc: "numbers, booleans and nested JSON keep their exact bytes",
			body: `[{"big":12345678901234567890,"price":1.50,"ok":true,"data":{"b": [1, 2], "a": null},"tags":["x","y"]}]`,
			want: "big,price,ok,data,tags\n12345678901234567890,1.50,true,\"{\"\"b\"\": [1, 2], \"\"a\"\": null}\",\"[\"\"x\"\",\"\"y\"\"]\"\n",
		},
		{
			desc: "null and a column missing from a row are empty fields",
			body: `[{"a":1,"b":null},{"a":2,"b":3},{"b":4}]`,
			want: "a,b\n1,\n2,3\n,4\n",
		},
		{
			desc: "a key repeated in a row keeps each value in its own column",
			body: `[{"age":19,"sum":7997,"sum":3},{"age":20,"sum":1350,"sum":4}]`,
			want: "age,sum,sum\n19,7997,3\n20,1350,4\n",
		},
		{
			desc: "column names are quoted when they need it",
			body: `[{"a,b":1,"c\"d":2}]`,
			want: "\"a,b\",\"c\"\"d\"\n1,2\n",
		},
		{
			desc: "a single row object becomes one row",
			body: `{"id":7,"name":"prest"}`,
			want: "id,name\n7,prest\n",
		},
		{
			desc: "an empty value in a single column is quoted so the row is not a blank line",
			body: `[{"n":null},{"n":""},{"n":1}]`,
			want: "n\n\"\"\n\"\"\n1\n",
		},
		{
			desc: "an array of rows with no rows is an empty file",
			body: ` [] `,
			want: "",
		},
		{
			desc: "rows with no columns are an empty file",
			body: `[{},{}]`,
			want: "",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			out, ok := renderCSV([]byte(tc.body))
			require.True(t, ok)
			require.Equal(t, tc.want, string(out))
		})
	}
}

func TestRenderCSV_ReadsBackAsTheSameRecords(t *testing.T) {
	t.Parallel()

	body := `[{"id":1,"note":"a, \"b\"\nc","n":null},{"id":2,"note":"","n":null}]`
	out, ok := renderCSV([]byte(body))
	require.True(t, ok)

	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"id", "note", "n"},
		{"1", "a, \"b\"\nc", ""},
		{"2", "", ""},
	}, records)
}

func TestRenderCSV_SingleColumnRowsAreKept(t *testing.T) {
	t.Parallel()

	// Unquoted, a row whose only value is null would be a blank line, which
	// CSV readers skip.
	out, ok := renderCSV([]byte(`[{"n":null},{"n":1},{"n":null}]`))
	require.True(t, ok)

	records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
	require.NoError(t, err)
	require.Equal(t, [][]string{{"n"}, {""}, {"1"}, {""}}, records)
}

func TestRenderCSV_NotRows(t *testing.T) {
	t.Parallel()

	rows := make([]string, 200)
	for i := range rows {
		rows[i] = fmt.Sprintf(`{"k%d":%d}`, i, i)
	}
	sparse := "[" + strings.Join(rows, ",") + "]"

	for _, body := range []string{
		``,
		`null`,
		`42`,
		`"text"`,
		`[1,2]`,
		`[{"a":1},2]`,
		`[[1,2]]`,
		`{"a":1}{"a":2}`,
		`[{"a":1}] trailing`,
		`not json`,
		sparse,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			_, ok := renderCSV([]byte(body))
			require.False(t, ok)
		})
	}
}
