package controllers_test

import (
	"net/http"
	"testing"

	"github.com/prest/prest/v2/integration/helpers"
	"github.com/prest/prest/v2/integration/testutils"
	"github.com/stretchr/testify/require"
)

const returningErrorsPath = "/prest-test/public/returning_errors"

// TestReturningStatementErrors: a write with _returning whose statement fails
// in the database answers 400, the same as the write without _returning. The
// query error used to be dropped and the nil rows dereferenced, so the request
// panicked into a 500 that carried the goroutine stack.
func TestReturningStatementErrors(t *testing.T) {
	base := helpers.ServerURL(t) + returningErrorsPath
	t.Cleanup(func() {
		testutils.DoRequest(t, base+"?id=$gt.0", nil, http.MethodDelete, http.StatusOK, "ReturningErrorsCleanup")
	})
	for _, code := range []string{"a", "b"} {
		testutils.DoRequest(t, base, map[string]any{"code": code, "qty": 1}, http.MethodPost, http.StatusCreated, "ReturningErrorsInsert")
	}

	cases := []struct {
		description string
		url         string
		method      string
		body        map[string]any
	}{
		{"PATCH breaking a unique constraint", "?code=b&_returning=*", http.MethodPatch, map[string]any{"code": "a"}},
		{"PUT breaking a check constraint", "?code=b&_returning=id", http.MethodPut, map[string]any{"qty": -1}},
		{"multi-row PATCH breaking a unique constraint on a later row", "?id=$gt.0&_returning=id", http.MethodPatch, map[string]any{"code": "same"}},
		{"DELETE with a filter value of the wrong type", "?id=abc&_returning=id", http.MethodDelete, nil},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			testutils.DoRequest(t, base+tc.url, tc.body, tc.method, http.StatusBadRequest, "ReturningErrors")
		})
	}

	// None of the failed statements changed a row.
	var rows []map[string]any
	testutils.DoRequestJSON(t, base+"?_order=code", nil, http.MethodGet, http.StatusOK, "ReturningErrorsUnchanged", &rows)
	require.Len(t, rows, 2)
	require.Equal(t, "a", rows[0]["code"])
	require.Equal(t, "b", rows[1]["code"])
	require.EqualValues(t, 1, rows[1]["qty"])

	// A write that succeeds still returns the changed rows.
	var updated []map[string]any
	testutils.DoRequestJSON(t, base+"?code=b&_returning=*", map[string]any{"qty": 2}, http.MethodPatch, http.StatusOK, "ReturningErrorsUpdated", &updated)
	require.Len(t, updated, 1)
	require.EqualValues(t, 2, updated[0]["qty"])
}
