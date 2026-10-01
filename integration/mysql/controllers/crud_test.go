package mysql_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
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

func TestMySQLBatchInsertCopy(t *testing.T) {
	// Batch insert with Prest-Batch-Method copy writes multiple rows.
	base := helpers.ServerURL(t)
	testutils.DoRequestWithHeaders(
		t,
		base+"/batch/shop/shop/items",
		[]map[string]any{{"name": "copy-a", "qty": 1}, {"name": "copy-b", "qty": 1}},
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

func TestMySQLRejectsUnsupportedOperators(t *testing.T) {
	// $tsquery and FULL JOIN are rejected on MySQL.
	base := helpers.ServerURL(t)
	testutils.DoRequest(t, base+"/shop/shop/items?body=$tsquery.hello", nil, http.MethodGet, http.StatusBadRequest, "tsquery")
	testutils.DoRequest(t, base+"/shop/shop/items?_join=FULL:tags:tags.id:$eq:items.id", nil, http.MethodGet, http.StatusBadRequest, "full join")
}
