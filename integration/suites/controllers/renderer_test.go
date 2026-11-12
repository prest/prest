package controllers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

type rowsResponse struct {
	Metadata []string `json:"metadata"`
	Rows     [][]any  `json:"rows"`
}

type columnsResponse struct {
	Metadata []string `json:"metadata"`
	Columns  [][]any  `json:"columns"`
}

// TestRowsAndColumnsRenderers: _renderer=rows and _renderer=columns return the
// same values as the default renderer, with the column names sent once in
// "metadata" instead of in every row object.
func TestRowsAndColumnsRenderers(t *testing.T) {
	base := helpers.ServerURL(t)
	const path = "/prest-test/public/test_group_by_table?_select=id,name,age&_order=id"

	// The default renderer is the reference: an array of row objects.
	var objects []map[string]any
	testutils.DoRequestJSON(t, base+path, nil, http.MethodGet, http.StatusOK, "RenderObjects", &objects)
	require.NotEmpty(t, objects)

	// rows: one array per row, its values in the order of metadata.
	var rows rowsResponse
	testutils.DoRequestJSON(t, base+path+"&_renderer=rows", nil, http.MethodGet, http.StatusOK, "RenderRows", &rows)
	require.ElementsMatch(t, []string{"id", "name", "age"}, rows.Metadata)
	require.Len(t, rows.Rows, len(objects))
	for r, object := range objects {
		require.Len(t, rows.Rows[r], len(rows.Metadata))
		for c, name := range rows.Metadata {
			require.Equal(t, object[name], rows.Rows[r][c], "row %d, column %s", r, name)
		}
	}

	// columns: one array per column, its values in row order.
	var columns columnsResponse
	testutils.DoRequestJSON(t, base+path+"&_renderer=columns", nil, http.MethodGet, http.StatusOK, "RenderColumns", &columns)
	require.Equal(t, rows.Metadata, columns.Metadata)
	require.Len(t, columns.Columns, len(columns.Metadata))
	for c, name := range columns.Metadata {
		require.Len(t, columns.Columns[c], len(objects))
		for r, object := range objects {
			require.Equal(t, object[name], columns.Columns[c][r], "row %d, column %s", r, name)
		}
	}
}

// TestRowsRendererResponseShapes: the rows renderer handles every shape a read
// can return, and leaves error responses as they are.
func TestRowsRendererResponseShapes(t *testing.T) {
	base := helpers.ServerURL(t)

	// An aggregate is a single row with a single column.
	var count rowsResponse
	testutils.DoRequestJSON(t, base+"/prest-test/public/test_group_by_table?_count=*&_renderer=rows", nil, http.MethodGet, http.StatusOK, "RenderRowsCount", &count)
	require.Equal(t, []string{"count"}, count.Metadata)
	require.Len(t, count.Rows, 1)
	require.Len(t, count.Rows[0], 1)

	// A table with no rows has no columns to report: empty metadata and rows,
	// not null, so clients can iterate without a nil check.
	testutils.DoRequest(t, base+"/prest-test/public/test_empty_table?_renderer=rows", nil, http.MethodGet, http.StatusOK, "RenderRowsEmpty",
		`{"metadata":[],"rows":[]}`)

	// An error is not tabular data: the usual {"error": ...} body comes back.
	testutils.DoRequest(t, base+"/prest-test/public/test_group_by_table?_select=nope&_renderer=rows", nil, http.MethodGet, http.StatusBadRequest, "RenderRowsError",
		`"error"`)
}

// getResponse sends a GET request and returns the status, the media type and
// the body of the response.
func getResponse(t *testing.T, url string) (status int, contentType, body string) {
	t.Helper()
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	byt, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(byt)
}

// msgpackAsJSON decodes a msgpack body and writes it back as JSON, so it can
// be compared with what the default renderer returns.
func msgpackAsJSON(t *testing.T, body string) string {
	t.Helper()
	var decoded any
	require.NoError(t, msgpack.Unmarshal([]byte(body), &decoded))
	asJSON, err := json.Marshal(decoded)
	require.NoError(t, err)
	return string(asJSON)
}

// TestMsgpackRenderer: _renderer=msgpack returns the same data as the default
// renderer, encoded as MessagePack, for every kind of response a read returns.
func TestMsgpackRenderer(t *testing.T) {
	base := helpers.ServerURL(t)

	for _, tc := range []struct {
		description string
		query       string
		status      int
	}{
		{"rows of a table", "test_group_by_table?_select=id,name,age&_order=id", http.StatusOK},
		{"nested jsonb values", "test_jsonb_bug?_order=id", http.StatusOK},
		{"array values", "testarray?_select=id,data", http.StatusOK},
		{"fractional numbers", "test_group_by_table?_select=age,avg:salary&_groupby=age&_order=age", http.StatusOK},
		{"an aggregate is a single row", "test_group_by_table?_count=*", http.StatusOK},
		{"an empty table is an empty array", "test_empty_table?_select=id", http.StatusOK},
		{"an error keeps its {\"error\": ...} body", "test_group_by_table?_select=nope", http.StatusBadRequest},
	} {
		t.Run(tc.description, func(t *testing.T) {
			url := base + "/prest-test/public/" + tc.query

			// The default renderer is the reference.
			status, _, want := getResponse(t, url)
			require.Equal(t, tc.status, status)

			// The msgpack renderer: same status, the msgpack media type, and a
			// body that decodes to the same values.
			status, contentType, body := getResponse(t, url+"&_renderer=msgpack")
			require.Equal(t, tc.status, status)
			require.Equal(t, "application/vnd.msgpack", contentType)
			require.JSONEq(t, want, msgpackAsJSON(t, body))
		})
	}
}
