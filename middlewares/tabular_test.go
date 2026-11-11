package middlewares

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderTable(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc    string
		body    string
		rows    string
		columns string
	}{
		{
			desc:    "array of rows keeps the column order of the query",
			body:    `[{"name":"joao","age":20,"id":1},{"name":"maria","age":19,"id":2}]`,
			rows:    `{"metadata":["name","age","id"],"rows":[["joao",20,1],["maria",19,2]]}`,
			columns: `{"metadata":["name","age","id"],"columns":[["joao","maria"],[20,19],[1,2]]}`,
		},
		{
			desc:    "values keep their exact bytes",
			body:    `[{"big":12345678901234567890,"price":1.50,"data":{"b":[1,2],"a":null},"tags":["x","y"],"html":"<b>&</b>"}]`,
			rows:    `{"metadata":["big","price","data","tags","html"],"rows":[[12345678901234567890,1.50,{"b":[1,2],"a":null},["x","y"],"<b>&</b>"]]}`,
			columns: `{"metadata":["big","price","data","tags","html"],"columns":[[12345678901234567890],[1.50],[{"b":[1,2],"a":null}],[["x","y"]],["<b>&</b>"]]}`,
		},
		{
			desc:    "a column missing from some rows is null there",
			body:    `[{"a":1},{"a":2,"b":3},{"b":4}]`,
			rows:    `{"metadata":["a","b"],"rows":[[1,null],[2,3],[null,4]]}`,
			columns: `{"metadata":["a","b"],"columns":[[1,2,null],[null,3,4]]}`,
		},
		{
			desc:    "a key repeated in a row keeps each value in its own column",
			body:    `[{"age":19,"sum":7997,"sum":3},{"age":20,"sum":1350,"sum":4}]`,
			rows:    `{"metadata":["age","sum","sum"],"rows":[[19,7997,3],[20,1350,4]]}`,
			columns: `{"metadata":["age","sum","sum"],"columns":[[19,20],[7997,1350],[3,4]]}`,
		},
		{
			desc:    "a single row object becomes one row",
			body:    `{"id":7,"name":"prest"}`,
			rows:    `{"metadata":["id","name"],"rows":[[7,"prest"]]}`,
			columns: `{"metadata":["id","name"],"columns":[[7],["prest"]]}`,
		},
		{
			desc:    "an empty array has no columns and no rows",
			body:    ` [] `,
			rows:    `{"metadata":[],"rows":[]}`,
			columns: `{"metadata":[],"columns":[]}`,
		},
		{
			desc:    "rows with no columns",
			body:    `[{},{}]`,
			rows:    `{"metadata":[],"rows":[[],[]]}`,
			columns: `{"metadata":[],"columns":[]}`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.rows, string(renderTable([]byte(tc.body), false)))
			require.Equal(t, tc.columns, string(renderTable([]byte(tc.body), true)))
		})
	}
}

func TestRenderTable_NonTabularBodyUnchanged(t *testing.T) {
	t.Parallel()

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
		`[{"a":1}`,
		`{"a":}`,
		`not json`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, body, string(renderTable([]byte(body), false)))
			require.Equal(t, body, string(renderTable([]byte(body), true)))
		})
	}
}
