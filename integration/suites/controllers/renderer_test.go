package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
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
