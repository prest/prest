package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

// TestArrayContainsFilter: $contains matches rows whose array column holds every
// listed element, whatever the array's element type. The parameter is untyped, so
// Postgres resolves it to the column's own array type.
func TestArrayContainsFilter(t *testing.T) {
	base := helpers.ServerURL(t)
	cleanupBatchDefaults(t, base, "a=contains-both", "a=contains-three")

	for name, nums := range map[string][]int{"contains-both": {1, 3}, "contains-three": {3}} {
		body := map[string]any{"a": name, "nums": nums}
		testutils.DoRequest(t, base+batchDefaultsPath, body, http.MethodPost, http.StatusCreated, "ArrayContainsInsert")
	}

	cases := []struct {
		description string
		query       string
		want        []string
	}{
		{"int[]: one element matches every row holding it", "nums=$contains.3&_order=a", []string{"contains-both", "contains-three"}},
		{"int[]: one element matches only the row holding it", "nums=$contains.1", []string{"contains-both"}},
		{"int[]: several elements must all be present", "nums=$contains.1,3", []string{"contains-both"}},
		{"int[]: an element no row holds matches nothing", "nums=$contains.2", nil},
		{"_or: goes through the same filter parsing", "_or=nums=$contains.1||a=$eq.contains-three&_order=a", []string{"contains-both", "contains-three"}},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			var rows []map[string]any
			testutils.DoRequestJSON(t, base+batchDefaultsPath+"?"+tc.query, nil, http.MethodGet, http.StatusOK, "ArrayContainsRead", &rows)
			got := []string(nil)
			for _, row := range rows {
				got = append(got, row["a"].(string))
			}
			require.Equal(t, tc.want, got)
		})
	}

	// varchar[] fixture: an array type that differs from the text[] a typed
	// parameter would default to, and whose elements are strings.
	const arrayPath = "/prest-test/public/testarray?id=100&data="
	var rows []map[string]any
	testutils.DoRequestJSON(t, base+arrayPath+"$contains.Gohan,Goten", nil, http.MethodGet, http.StatusOK, "ArrayContainsVarchar", &rows)
	require.Len(t, rows, 1)
	testutils.DoRequestJSON(t, base+arrayPath+"$contains.Goku", nil, http.MethodGet, http.StatusOK, "ArrayContainsVarcharMiss", &rows)
	require.Empty(t, rows)

	// A non-array column has no @> for an array parameter: the request fails
	// instead of silently matching nothing.
	testutils.DoRequest(t, base+"/prest-test/public/testarray?id=$contains.100", nil, http.MethodGet, http.StatusBadRequest, "ArrayContainsScalarColumn")
}
