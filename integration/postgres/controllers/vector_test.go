package controllers_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// getJSONArray performs a GET and decodes the pREST JSON array response so the
// test can assert on row order (testutils.DoRequest only does substring checks,
// which cannot prove nearest-first ordering).
func getJSONArray(t *testing.T, url string) []map[string]interface{} {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("X-Application", "prest")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "unexpected status; body: %s", string(body))

	var rows []map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &rows), "body: %s", string(body))
	return rows
}

// names extracts the "name" column in row order.
func names(rows []map[string]interface{}) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if n, ok := r["name"].(string); ok {
			out = append(out, n)
		}
	}
	return out
}

// TestVectorKNNOrdering verifies the _korder pgvector nearest-neighbor ordering.
// Seed distances from [1,0,0] (L2): alpha=0, delta≈0.14, beta≈1.41, gamma≈1.41.
func TestVectorKNNOrdering(t *testing.T) {
	base := helpers.ServerURL(t)

	// L2 nearest-first: alpha (exact match) then delta must lead the result set.
	url := fmt.Sprintf("%s/prest-test/public/vector_items?_korder=embedding:l2:[1,0,0]", base)
	rows := getJSONArray(t, url)
	got := names(rows)
	require.GreaterOrEqual(t, len(got), 4, "expected all seeded rows")
	require.Equal(t, "alpha", got[0], "closest vector must be first")
	require.Equal(t, "delta", got[1], "second closest vector must be second")

	// Cosine distance ranks by direction; alpha ([1,0,0]) is the closest direction.
	url = fmt.Sprintf("%s/prest-test/public/vector_items?_korder=embedding:cosine:[1,0,0]", base)
	rows = getJSONArray(t, url)
	require.Equal(t, "alpha", names(rows)[0], "cosine nearest must be alpha")
}

// TestVectorKNNCombinedWithOrder verifies _korder composes with a normal _order
// term in a single ORDER BY without breaking either clause.
func TestVectorKNNCombinedWithOrder(t *testing.T) {
	base := helpers.ServerURL(t)

	// _order=name (tie-break) plus KNN distance ordering — request must succeed.
	url := fmt.Sprintf("%s/prest-test/public/vector_items?_order=name&_korder=embedding:l2:[1,0,0]", base)
	rows := getJSONArray(t, url)
	require.GreaterOrEqual(t, len(rows), 4)
}

// TestVectorThresholdFilter verifies the :vecdist distance-threshold WHERE filter.
func TestVectorThresholdFilter(t *testing.T) {
	base := helpers.ServerURL(t)

	// Only alpha (0) and delta (≈0.14) are within L2 distance 0.5 of [1,0,0].
	url := fmt.Sprintf("%s/prest-test/public/vector_items?embedding:vecdist=l2:lt:[1,0,0]:0.5", base)
	rows := getJSONArray(t, url)
	got := names(rows)
	require.ElementsMatch(t, []string{"alpha", "delta"}, got,
		"threshold must exclude far vectors beta and gamma")

	// A tiny threshold keeps only the exact match.
	url = fmt.Sprintf("%s/prest-test/public/vector_items?embedding:vecdist=l2:lte:[1,0,0]:0.0001", base)
	rows = getJSONArray(t, url)
	require.Equal(t, []string{"alpha"}, names(rows))
}

// TestVectorSecurity_RejectsInjection is the adversarial suite: every payload is
// crafted to break out of the intended SQL context. All must fail closed with
// 400 (never 200, never 500), and the vector_items table must remain intact
// afterwards — proving no injected DDL/DML ever executed.
func TestVectorSecurity_RejectsInjection(t *testing.T) {
	base := helpers.ServerURL(t)
	table := fmt.Sprintf("%s/prest-test/public/vector_items", base)

	// %3B is an encoded ';' so it survives as a value byte instead of being
	// stripped as a query-pair separator — the strongest injection vector.
	cases := []struct {
		description string
		query       string
	}{
		{
			description: "metric field carries encoded SQL — whitelist rejects it",
			// _korder=embedding:l2; DROP TABLE vector_items:[1,0,0]
			query: "?_korder=embedding:l2%3B%20DROP%20TABLE%20vector_items:[1,0,0]",
		},
		{
			description: "column field attempts quote break-out — ident validation rejects it",
			// _korder=embedding"); DROP TABLE vector_items; --:l2:[1,0,0]
			query: "?_korder=embedding%22%29%3B%20DROP%20TABLE%20vector_items%3B%20--:l2:[1,0,0]",
		},
		{
			description: "vector literal appends a statement — reconstruction rejects it",
			// _korder=embedding:l2:[1,0,0]'; DROP TABLE vector_items; --
			query: "?_korder=embedding:l2:%5B1%2C0%2C0%5D%27%3B%20DROP%20TABLE%20vector_items%3B%20--",
		},
		{
			description: "vector element is not numeric — rejected before SQL",
			query:       "?_korder=embedding:l2:[1,abc,0]",
		},
		{
			description: "threshold carries encoded SQL — ParseFloat rejects it",
			// embedding:vecdist=l2:lt:[1,0,0]:0.5; DROP TABLE vector_items
			query: "?embedding:vecdist=l2:lt:[1,0,0]:0.5%3B%20DROP%20TABLE%20vector_items",
		},
		{
			description: "vecdist comparison smuggles LIKE — non-comparison operator rejected",
			query:       "?embedding:vecdist=l2:like:[1,0,0]:0.5",
		},
		{
			description: "vecdist vector break-out attempt — reconstruction rejects it",
			// embedding:vecdist=l2:lt:[1,0,0]'::text); DROP TABLE vector_items; --:0.5
			query: "?embedding:vecdist=l2:lt:%5B1%2C0%2C0%5D%27%3A%3Atext%29%3B%20DROP%20TABLE%20vector_items%3B%20--:0.5",
		},
		{
			description: "unknown vector metric is rejected",
			query:       "?_korder=embedding:hamming:[1,0,0]",
		},
	}

	for _, tc := range cases {
		t.Log(tc.description)
		// Every malicious request must fail closed with 400 Bad Request.
		testutils.DoRequest(t, table+tc.query, nil, http.MethodGet,
			http.StatusBadRequest, tc.description)
	}

	// Proof of no side effects: the table still exists with all 4 seeded rows.
	// If any injection had executed a DROP, this GET would 404 or return fewer rows.
	rows := getJSONArray(t, table+"?_order=id")
	require.Equal(t, []string{"alpha", "beta", "gamma", "delta"}, names(rows),
		"vector_items must be untouched after every injection attempt")
}

// TestVectorDimensionMismatch verifies a well-formed but wrong-dimension vector
// fails safely at the database (400) rather than crashing or corrupting state.
func TestVectorDimensionMismatch(t *testing.T) {
	base := helpers.ServerURL(t)

	// Column is vector(3); a 2-dim query vector must be rejected by Postgres.
	url := fmt.Sprintf("%s/prest-test/public/vector_items?_korder=embedding:l2:[1,0]", base)
	testutils.DoRequest(t, url, nil, http.MethodGet,
		http.StatusBadRequest, "dimension mismatch must fail safely")
}
