package mysql_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

func TestMySQLSchemasAndTables(t *testing.T) {
	// List user databases. shop and other are visible; server schemas stay hidden.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/databases", nil, http.MethodGet, http.StatusOK, "databases", "shop", "other")
	testutils.DoRequest(t, base+"/schemas", nil, http.MethodGet, http.StatusOK, "schemas", "shop")
	testutils.DoRequest(t, base+"/shop/shop", nil, http.MethodGet, http.StatusOK, "tables", "items")
}

func TestMySQLFiltersAndPagination(t *testing.T) {
	// Filter the seeded row with $eq, $in, $like, and $ilike, then paginate.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items?name=$eq.ada", nil, http.MethodGet, http.StatusOK, "eq", `"name":"ada"`)
	testutils.DoRequest(t, base+"/shop/shop/items?qty=$in.2,9", nil, http.MethodGet, http.StatusOK, "in", "ada")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$like.ad%25", nil, http.MethodGet, http.StatusOK, "like", "ada")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$ilike.AD%25", nil, http.MethodGet, http.StatusOK, "ilike", "ada")
	testutils.DoRequest(t, base+"/shop/shop/items?_page=1&_page_size=1", nil, http.MethodGet, http.StatusOK, "page", "ada")
}

func TestMySQLInsertUpdateDelete(t *testing.T) {
	// Insert a row, read it back, update with rows_affected, then delete.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items", map[string]any{"name": "bea", "qty": 4}, http.MethodPost, http.StatusCreated, "insert", "bea")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$eq.bea", nil, http.MethodGet, http.StatusOK, "read inserted", "bea")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$eq.bea", map[string]any{"qty": 9}, http.MethodPatch, http.StatusOK, "update", "rows_affected")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$eq.bea&_returning=name&_returning=qty", map[string]any{"qty": 8}, http.MethodPatch, http.StatusOK, "returning", "bea")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$eq.bea", nil, http.MethodDelete, http.StatusOK, "delete", "rows_affected")
}

func TestMySQLJSONObjectInsertAndPatch(t *testing.T) {
	// JSON objects in the body bind as utf8mb4 strings, so the default
	// interpolateParams mode writes them into a JSON column (no error 3144).
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items", map[string]any{"name": "socks", "qty": 5, "meta": map[string]any{"kind": "apparel"}}, http.MethodPost, http.StatusCreated, "insert json", "apparel")
	testutils.DoRequest(t, base+"/shop/shop/items?meta->>kind:jsonb=apparel", nil, http.MethodGet, http.StatusOK, "filter json", "socks")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$eq.socks", map[string]any{"meta": map[string]any{"kind": "shoes"}}, http.MethodPatch, http.StatusOK, "patch json", "rows_affected")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$eq.socks", nil, http.MethodGet, http.StatusOK, "read patched", "shoes")
	testutils.DoRequest(t, base+"/shop/shop/items?name=$eq.socks", nil, http.MethodDelete, http.StatusOK, "cleanup", "rows_affected")
}

func TestMySQLBatchInsertCopy(t *testing.T) {
	// Batch insert with Prest-Batch-Method copy writes multiple rows, one with a JSON object.
	base := helpers.ServerURL(t)
	testutils.DoRequestWithHeaders(
		t,
		base+"/batch/shop/shop/items",
		[]map[string]any{{"name": "copy-a", "qty": 1}, {"name": "copy-b", "qty": 1, "meta": map[string]any{"kind": "batch"}}},
		http.MethodPost,
		http.StatusCreated,
		"batch copy",
		map[string]string{"Prest-Batch-Method": "copy"},
		"copy-a",
		"copy-b",
	)
}

func TestMySQLShowColumnsAndMissingTable(t *testing.T) {
	// Show columns uses Postgres-shaped keys. An unknown table is 404.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/show/shop/shop/items", nil, http.MethodGet, http.StatusOK, "show", "column_name", "is_generated")
	testutils.DoRequest(t, base+"/shop/shop/no_such_table", nil, http.MethodGet, http.StatusNotFound, "missing")
}

func TestMySQLHealthReportsAdapter(t *testing.T) {
	// Probe liveness on the MySQL server.
	// Expected to succeed with HTTP 200 and name the mysql adapter.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/_health", nil, http.MethodGet, http.StatusOK, "health adapter", `"adapter":"mysql"`)
}

func TestMySQLRejectsUnsupportedOperators(t *testing.T) {
	// $tsquery and FULL JOIN are rejected on MySQL.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items?body=$tsquery.hello", nil, http.MethodGet, http.StatusBadRequest, "tsquery")
	testutils.DoRequest(t, base+"/shop/shop/items?_join=FULL:tags:tags.id:$eq:items.id", nil, http.MethodGet, http.StatusBadRequest, "full join")
}

func TestMySQLCountKey(t *testing.T) {
	// _count returns the key "count", the same as Postgres, not "COUNT(*)".
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items?_count=*", nil, http.MethodGet, http.StatusOK, "count", `"count":`)
}

func TestMySQLNullAndBoolOperatorsWithoutDot(t *testing.T) {
	// Value-less operators work with and without a trailing dot. The seeded
	// NULL-name row has qty 3 and flag FALSE; ada has flag TRUE.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items?name=$null&_select=qty", nil, http.MethodGet, http.StatusOK, "null no dot", `"qty":3`)
	testutils.DoRequest(t, base+"/shop/shop/items?name=$null.&_select=qty", nil, http.MethodGet, http.StatusOK, "null with dot", `"qty":3`)
	testutils.DoRequest(t, base+"/shop/shop/items?name=$notnull&_select=name", nil, http.MethodGet, http.StatusOK, "notnull no dot", `"name":"ada"`)
	// Before the fix ?flag=$true returned the false rows; now it returns ada.
	testutils.DoRequest(t, base+"/shop/shop/items?flag=$true&_select=name", nil, http.MethodGet, http.StatusOK, "true no dot", `"name":"ada"`)
	testutils.DoRequest(t, base+"/shop/shop/items?flag=$false&_select=qty", nil, http.MethodGet, http.StatusOK, "false no dot", `"qty":3`)

	// $true must not match the false row: decode and check every returned flag.
	var rows []map[string]any
	testutils.DoRequestJSON(t, base+"/shop/shop/items?flag=$true&_select=flag", nil, http.MethodGet, http.StatusOK, "true rows", &rows)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		require.EqualValues(t, 1, r["flag"], "flag=$true returned a non-true row: %v", r)
	}
}

func TestMySQLBinaryAsHex(t *testing.T) {
	// VARBINARY is returned like Postgres bytea: "\x" plus lowercase hex.
	// The JSON body escapes the backslash, so the raw bytes read "\\xdeadbeef".
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items?name=ada&_select=raw", nil, http.MethodGet, http.StatusOK, "binary hex", `"raw":"\\xdeadbeef"`)

	var rows []map[string]any
	testutils.DoRequestJSON(t, base+"/shop/shop/items?name=ada&_select=raw", nil, http.MethodGet, http.StatusOK, "binary hex decoded", &rows)
	require.Len(t, rows, 1)
	require.Equal(t, `\xdeadbeef`, rows[0]["raw"])
}

func TestMySQLDateTimeRoundTrip(t *testing.T) {
	// DATETIME is rendered without a zone and DATE without a time part, and the
	// DATETIME string is accepted back unchanged by a PATCH.
	base := helpers.ServerURL(t)
	var rows []map[string]any
	testutils.DoRequestJSON(t, base+"/shop/shop/items?name=ada&_select=created_at,day", nil, http.MethodGet, http.StatusOK, "read datetime", &rows)
	require.Len(t, rows, 1)
	require.Equal(t, "2026-10-07T13:45:01.123", rows[0]["created_at"])
	require.Equal(t, "2026-10-07", rows[0]["day"])

	// Writing the returned value back must succeed (no 'Z' suffix MySQL rejects).
	testutils.DoRequest(t, base+"/shop/shop/items?name=ada", map[string]any{"created_at": rows[0]["created_at"]}, http.MethodPatch, http.StatusOK, "patch datetime", "rows_affected")
}

func TestMySQLNoAccessSchemaIs404(t *testing.T) {
	// An unknown schema makes MySQL answer 1142/1044/1049 ("denied to user
	// 'prest'@'host'"). pREST returns 404 and must not echo the server text.
	base := helpers.ServerURL(t)
	resp, err := http.Get(base + "/shop/no_such_schema/items")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "body: %s", body)
	require.NotContains(t, string(body), "@")
}

func TestMySQLAggregateDefaultAlias(t *testing.T) {
	// An aggregate without an explicit alias gets the Postgres-style key "sum".
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items?_select=name,sum:qty&_groupby=name", nil, http.MethodGet, http.StatusOK, "sum key", `"sum":`)
}

func TestMySQLBatchHeterogeneousKeys(t *testing.T) {
	// Batch records with different keys keep every column; a key missing from a
	// record takes the column DEFAULT (qty DEFAULT 0) instead of being dropped.
	base := helpers.ServerURL(t)
	t.Cleanup(func() {
		testutils.DoRequest(t, base+"/shop/shop/items?name=$in.h1,h2", nil, http.MethodDelete, http.StatusOK, "cleanup")
	})
	testutils.DoRequest(t, base+"/batch/shop/shop/items",
		[]map[string]any{{"name": "h1", "qty": 5}, {"name": "h2", "meta": map[string]any{"k": 1}}},
		http.MethodPost, http.StatusCreated, "batch heterogeneous", "h1", "h2")
	testutils.DoRequest(t, base+"/shop/shop/items?name=h2", nil, http.MethodGet, http.StatusOK, "h2 default qty", `"qty":0`, `"k":1`)
	testutils.DoRequest(t, base+"/shop/shop/items?name=h1", nil, http.MethodGet, http.StatusOK, "h1 qty", `"qty":5`)
}

func TestMySQLLargeIntegers(t *testing.T) {
	// 2^53+1 cannot be represented as float64; the body is decoded with
	// UseNumber, so the value is stored and returned exactly.
	base := helpers.ServerURL(t)
	t.Cleanup(func() {
		testutils.DoRequest(t, base+"/shop/shop/items?name=bigint", nil, http.MethodDelete, http.StatusOK, "cleanup")
	})
	testutils.DoRequest(t, base+"/shop/shop/items", map[string]any{"name": "bigint", "big": int64(9007199254740993)}, http.MethodPost, http.StatusCreated, "insert big", "9007199254740993")
	testutils.DoRequest(t, base+"/shop/shop/items?name=bigint", nil, http.MethodGet, http.StatusOK, "read big", `"big":9007199254740993`)
}
