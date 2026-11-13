package controllers_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// TestCSVRenderer: _renderer=csv returns the same rows as the default renderer
// as CSV, with a header record of column names, as text/csv.
func TestCSVRenderer(t *testing.T) {
	base := helpers.ServerURL(t)
	const path = "/prest-test/public/test_group_by_table?_select=id,name,age&_order=id"

	// The default renderer is the reference: an array of row objects.
	var objects []map[string]any
	testutils.DoRequestJSON(t, base+path, nil, http.MethodGet, http.StatusOK, "RenderObjects", &objects)
	require.NotEmpty(t, objects)

	status, contentType, body := getResponse(t, base+path+"&_renderer=csv")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "text/csv; charset=utf-8", contentType)

	records, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, len(objects)+1)
	header := records[0]
	require.ElementsMatch(t, []string{"id", "name", "age"}, header)
	for r, object := range objects {
		for c, name := range header {
			require.Equal(t, fmt.Sprint(object[name]), records[r+1][c], "row %d, column %s", r, name)
		}
	}
}

// TestCSVRendererResponseShapes: the csv renderer handles an aggregate, an
// empty table and nested jsonb, and leaves error responses as JSON.
func TestCSVRendererResponseShapes(t *testing.T) {
	base := helpers.ServerURL(t)

	// An aggregate is a single row with a single column.
	status, contentType, body := getResponse(t, base+"/prest-test/public/test_group_by_table?_count=*&_renderer=csv")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "text/csv; charset=utf-8", contentType)
	records, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, []string{"count"}, records[0])

	// A table with no rows has no columns to name: an empty file.
	status, contentType, body = getResponse(t, base+"/prest-test/public/test_empty_table?_renderer=csv")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "text/csv; charset=utf-8", contentType)
	require.Empty(t, body)

	// A jsonb value is written as its JSON text, which parses back to the
	// value the default renderer returns.
	var objects []map[string]any
	testutils.DoRequestJSON(t, base+"/prest-test/public/test_jsonb_bug?_order=id", nil, http.MethodGet, http.StatusOK, "RenderJSONB", &objects)
	require.NotEmpty(t, objects)
	status, _, body = getResponse(t, base+"/prest-test/public/test_jsonb_bug?_order=id&_renderer=csv")
	require.Equal(t, http.StatusOK, status)
	records, err = csv.NewReader(strings.NewReader(body)).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, len(objects)+1)
	for c, name := range records[0] {
		for r, object := range objects {
			if nested, ok := object[name].(map[string]any); ok {
				want, err := json.Marshal(nested)
				require.NoError(t, err)
				require.JSONEq(t, string(want), records[r+1][c], "row %d, column %s", r, name)
			}
		}
	}

	// An error is not tabular data: the usual {"error": ...} body comes back.
	status, contentType, body = getResponse(t, base+"/prest-test/public/test_group_by_table?_select=nope&_renderer=csv")
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "application/json", contentType)
	require.Contains(t, body, `"error"`)
}
