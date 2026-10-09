package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

const batchDefaultsPath = "/prest-test/public/batch_defaults"

// cleanupBatchDefaults deletes the rows a test created, filtered by query.
func cleanupBatchDefaults(t *testing.T, base string, queries ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, q := range queries {
			testutils.DoRequest(t, base+batchDefaultsPath+"?"+q, nil, "DELETE", http.StatusOK, "BatchDefaultsCleanup")
		}
	})
}

// TestBatchHeterogeneousKeysUseDefaults: records with different keys keep every
// column (union of keys) and an omitted key takes the column DEFAULT, so a
// NOT NULL column with a default still inserts. COPY cannot express DEFAULT.
func TestBatchHeterogeneousKeysUseDefaults(t *testing.T) {
	base := helpers.ServerURL(t)
	cleanupBatchDefaults(t, base, "a=x1", "b=y1", "c=given", "a=x2", "b=y2")

	body := []map[string]interface{}{{"a": "x1", "c": "given"}, {"b": "y1"}}
	testutils.DoRequest(t, base+"/batch"+batchDefaultsPath, body, "POST", http.StatusCreated, "BatchHeterogeneousInsert")

	var first, second []map[string]interface{}
	testutils.DoRequestJSON(t, base+batchDefaultsPath+"?a=x1", nil, "GET", http.StatusOK, "BatchFirstRecord", &first)
	require.Len(t, first, 1)
	require.Equal(t, "dflt", first[0]["b"], "omitted nullable key takes the column default")
	require.Equal(t, "given", first[0]["c"])

	testutils.DoRequestJSON(t, base+batchDefaultsPath+"?b=y1", nil, "GET", http.StatusOK, "BatchSecondRecord", &second)
	require.Len(t, second, 1, "the second record's column is kept")
	require.Nil(t, second[0]["a"])
	require.Equal(t, "req", second[0]["c"], "omitted NOT NULL key takes the column default")

	copyBody := []map[string]interface{}{{"a": "x2"}, {"b": "y2"}}
	testutils.DoRequestWithHeaders(t, base+"/batch"+batchDefaultsPath, copyBody, "POST", http.StatusBadRequest,
		"BatchCopyHeterogeneous", map[string]string{"Prest-Batch-Method": "copy"}, "same keys")
}

// TestLargeIntegerPrecision: integers above 2^53 must reach a bigint column
// exactly (bodies are decoded with UseNumber, not float64).
func TestLargeIntegerPrecision(t *testing.T) {
	base := helpers.ServerURL(t)
	cleanupBatchDefaults(t, base, "big=9007199254740993")

	body := map[string]interface{}{"big": int64(9007199254740993)}
	testutils.DoRequest(t, base+batchDefaultsPath, body, "POST", http.StatusCreated, "InsertLargeInteger", "9007199254740993")
	testutils.DoRequest(t, base+batchDefaultsPath+"?big=9007199254740993", nil, "GET", http.StatusOK, "SelectLargeInteger", "9007199254740993")
}

// TestNumericArrayInsert: a JSON array of numbers into an int[] column is
// stored as {1,2} (previously serialized as "{,}").
func TestNumericArrayInsert(t *testing.T) {
	base := helpers.ServerURL(t)
	cleanupBatchDefaults(t, base, "a=arr")

	body := map[string]interface{}{"a": "arr", "nums": []int{1, 2}}
	testutils.DoRequest(t, base+batchDefaultsPath, body, "POST", http.StatusCreated, "InsertNumericArray", `"nums":[1,2]`)
	// The select is rendered by Postgres JSON (spaced), so decode instead of matching text.
	var rows []map[string]interface{}
	testutils.DoRequestJSON(t, base+batchDefaultsPath+"?a=arr", nil, "GET", http.StatusOK, "SelectNumericArray", &rows)
	require.Len(t, rows, 1)
	require.Equal(t, []interface{}{float64(1), float64(2)}, rows[0]["nums"])
}

// TestDatabasesListsAliasesInRegistryMode: with a [[databases]] registry,
// /databases lists the configured aliases with their physical database names.
func TestDatabasesListsAliasesInRegistryMode(t *testing.T) {
	base := helpers.MultiClusterServerURL(t)

	testutils.DoRequest(t, base+"/databases", nil, "GET", http.StatusOK, "RegistryDatabases",
		`"datname":"prest-test"`,
		`"datname":"secondary-db"`,
		`"physical_name":"secondary-cluster"`,
	)
}
