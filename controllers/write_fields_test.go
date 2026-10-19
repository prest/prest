package controllers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBodyKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		body        string
		batch       bool
		keys        []string
		wantErr     bool
	}{
		{"object keys come back sorted", `{"b":1,"a":2}`, false, []string{"a", "b"}, false},
		{"batch keys are the union of every record", `[{"a":1},{"b":2,"a":3}]`, true, []string{"a", "b"}, false},
		// The builders decode only the first JSON value, so the gate must too:
		// a key in a trailing value never reaches SQL and must not be required.
		{"trailing value is ignored like the builders do", `{"a":1} {"role":2}`, false, []string{"a"}, false},
		{"duplicate keys collapse", `{"a":1,"a":2}`, false, []string{"a"}, false},
		{"keys are case-sensitive", `{"Name":1,"name":2}`, false, []string{"Name", "name"}, false},
		{"null body has no keys", `null`, false, nil, false},
		{"truncated object is an error", `{`, false, nil, true},
		{"empty body is an error", ``, false, nil, true},
		{"batch rejects an object body", `{"a":1}`, true, nil, true},
		{"object rejects an array body", `[{"a":1}]`, false, nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))

			keys, err := bodyKeys(req, tc.batch)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.keys, keys)

			// The builder decodes r.Body after the gate, so it must see the
			// exact bytes the client sent.
			rest, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, tc.body, string(rest))
		})
	}
}

func TestFilterColumns(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		body        string
		columns     []string
		want        string
		wantErr     error
		anyErr      bool
	}{
		{"object drops unreadable columns and keeps order",
			`{"id":1,"role":"admin","name":"x"}`, []string{"name", "id"}, `{"id":1,"name":"x"}`, nil, false},
		{"array filters every row",
			`[{"id":1,"role":"a"},{"id":2,"role":"b"}]`, []string{"id"}, `[{"id":1},{"id":2}]`, nil, false},
		{"no readable column leaves an empty object",
			`{"id":1,"role":"admin"}`, nil, `{}`, nil, false},
		{"no readable column leaves empty rows",
			`[{"id":1},{"id":2}]`, nil, `[{},{}]`, nil, false},
		{"empty array stays empty", `[]`, []string{"id"}, `[]`, nil, false},
		{"null passes through", `null`, []string{"id"}, `null`, nil, false},
		{"empty body passes through", ``, []string{"id"}, ``, nil, false},
		{"whitespace body passes through", " \n", []string{"id"}, " \n", nil, false},
		// Values are copied byte for byte, never re-encoded through float64.
		{"large integers keep every digit",
			`{"id":12345678901234567890,"role":1}`, []string{"id"}, `{"id":12345678901234567890}`, nil, false},
		{"nested JSON values are kept verbatim",
			`{"name":{"x":[1, 2]},"role":"a"}`, []string{"name"}, `{"name":{"x":[1, 2]}}`, nil, false},
		{"keys needing escapes round-trip",
			`{"a\"b":1,"role":2}`, []string{`a"b`}, `{"a\"b":1}`, nil, false},
		{"top-level number is rejected", `1`, []string{"id"}, "", errUnexpectedResponseShape, false},
		{"top-level string is rejected", `"x"`, []string{"id"}, "", errUnexpectedResponseShape, false},
		{"array of scalars is rejected", `[1]`, []string{"id"}, "", errUnexpectedResponseShape, false},
		{"trailing data is rejected", `{} {}`, []string{"id"}, "", nil, true},
		{"invalid JSON is rejected", `{"id":`, []string{"id"}, "", nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			got, err := filterColumns([]byte(tc.body), tc.columns)
			switch {
			case tc.wantErr != nil:
				require.ErrorIs(t, err, tc.wantErr)
				return
			case tc.anyErr:
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))
			if strings.TrimSpace(tc.want) != "" {
				require.True(t, json.Valid(got), "filtered body is not valid JSON: %s", got)
			}
		})
	}
}
