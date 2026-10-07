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

// TestBatchHeterogeneousKeysUseDefaults: a batch whose records carry different
// keys keeps every record's columns and fills omitted ones with the column
// DEFAULT; COPY cannot express DEFAULT, so it rejects mixed keys with 400.
func TestBatchHeterogeneousKeysUseDefaults(t *testing.T) {
	base := helpers.ServerURL(t)
	cleanupBatchDefaults(t, base, "a=x1", "b=y1", "a=x2", "b=y2")

	body := []map[string]interface{}{{"a": "x1"}, {"b": "y1"}}
	testutils.DoRequest(t, base+"/batch"+batchDefaultsPath, body, "POST", http.StatusCreated, "BatchHeterogeneousInsert")

	testutils.DoRequest(t, base+batchDefaultsPath+"?a=x1", nil, "GET", http.StatusOK, "BatchDefaultApplied", `"dflt"`)
	testutils.DoRequest(t, base+batchDefaultsPath+"?b=y1", nil, "GET", http.StatusOK, "BatchSecondRecordKept", `"y1"`)

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
