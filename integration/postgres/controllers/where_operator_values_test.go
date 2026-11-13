package controllers_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

const whereOperatorValuesPath = "/prest-test/public/where_operator_values"

// TestWhereOperatorInsideValue: only a leading $op is an operator, so a "$word."
// sequence inside a filter value reaches the database unchanged. When it was
// stripped, "a$bc.d" matched the row "ad", on writes as well as on reads.
func TestWhereOperatorInsideValue(t *testing.T) {
	base := helpers.ServerURL(t)
	filter := func(value string) string {
		return base + whereOperatorValuesPath + "?name=" + url.QueryEscape(value)
	}
	t.Cleanup(func() {
		testutils.DoRequest(t, base+whereOperatorValuesPath+"?id=$gt.0", nil, http.MethodDelete, http.StatusOK, "WhereOperatorValuesCleanup")
	})
	for _, name := range []string{"a$bc.d", "ad", "cost$usd", "$gt.5"} {
		testutils.DoRequest(t, base+whereOperatorValuesPath, map[string]any{"name": name}, http.MethodPost, http.StatusCreated, "WhereOperatorValuesInsert")
	}

	cases := []struct {
		description string
		value       string
		want        string
	}{
		{"operator-like text in the middle of an $eq value matches that exact row", "$eq.a$bc.d", "a$bc.d"},
		{"operator-like suffix on a bare value is data, not an invalid operator", "cost$usd", "cost$usd"},
		{"$eq escapes a value that starts like an operator", "$eq.$gt.5", "$gt.5"},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			var rows []map[string]any
			testutils.DoRequestJSON(t, filter(tc.value), nil, http.MethodGet, http.StatusOK, "WhereOperatorValuesRead", &rows)
			require.Len(t, rows, 1)
			require.Equal(t, tc.want, rows[0]["name"])
		})
	}

	// DELETE by the same filter removes only the exact row; "ad" must survive the
	// write that would have hit it when the value was stripped to "ad".
	testutils.DoRequest(t, filter("$eq.a$bc.d"), nil, http.MethodDelete, http.StatusOK, "WhereOperatorValuesDelete")
	var survivors []map[string]any
	testutils.DoRequestJSON(t, filter("$eq.ad"), nil, http.MethodGet, http.StatusOK, "WhereOperatorValuesSurvivor", &survivors)
	require.Len(t, survivors, 1)
	var deleted []map[string]any
	testutils.DoRequestJSON(t, filter("$eq.a$bc.d"), nil, http.MethodGet, http.StatusOK, "WhereOperatorValuesDeleted", &deleted)
	require.Empty(t, deleted)
}
