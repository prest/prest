package mysql

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// Value-less operators must work with or without a trailing dot, matching
// Postgres; otherwise `$null` is compared as the literal string '$null'.
func TestWhereKeyAndValueOperators(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, key, value, want string
		args                   []interface{}
	}{
		{"null", "c", "$null", "`c` IS NULL", nil},
		{"null dot", "c", "$null.", "`c` IS NULL", nil},
		{"notnull", "c", "$notnull", "`c` IS NOT NULL", nil},
		{"true", "c", "$true", "`c` IS TRUE", nil},
		{"true dot", "c", "$true.", "`c` IS TRUE", nil},
		{"false", "c", "$false", "`c` IS FALSE", nil},
		{"nottrue", "c", "$nottrue", "`c` IS NOT TRUE", nil},
		{"notfalse", "c", "$notfalse", "`c` IS NOT FALSE", nil},
		{"eq", "c", "$eq.x", "`c` = ?", []interface{}{"x"}},
		{"in", "c", "$in.a,b", "`c` IN (?,?)", []interface{}{"a", "b"}},
		{"dollar amount stays equality", "c", "$100", "`c` = ?", []interface{}{"$100"}},
		{"plain value", "c", "abc", "`c` = ?", []interface{}{"abc"}},
		{"operator only matched at start", "c", "x$eq.y", "`c` = ?", []interface{}{"x$eq.y"}},
		{"json null", "meta->>k:jsonb", "$null", "`meta`->>'$.k' IS NULL", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pid := 0
			got, args, err := whereKeyAndValue(tc.key, tc.value, &pid)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.args, args)
		})
	}
}

// Unknown letters-only operators are rejected rather than silently compared.
func TestWhereKeyAndValueBogusOperator(t *testing.T) {
	t.Parallel()
	pid := 0
	_, _, err := whereKeyAndValue("c", "$bogus", &pid)
	require.ErrorIs(t, err, errInvalidOperator)
}

// The _or path shares the same operator parsing.
func TestWhereByRequestOrValueLessOperator(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	where, args, err := a.WhereByRequest(req("/t?_or="+url.QueryEscape("a=$null||b=$eq.1")), 1)
	require.NoError(t, err)
	require.Equal(t, "(`a` IS NULL OR `b` = ?)", where)
	require.Equal(t, []interface{}{"1"}, args)
}
