package queryguard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A policy with no rule enabled enforces nothing, so callers can skip wrapping.
func TestPolicyIsZero(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		policy Policy
		want   bool
	}{
		"empty":                {Policy{}, true},
		"allow tables only":    {Policy{AllowTables: []string{"countries"}}, true},
		"non positive limits":  {Policy{MaxCost: -1, MaxRows: 0, MaxJoins: -3}, true},
		"reject seq scan":      {Policy{RejectSeqScan: true}, false},
		"reject parallel scan": {Policy{RejectParallelSeqScan: true}, false},
		"max cost":             {Policy{MaxCost: 1}, false},
		"max rows":             {Policy{MaxRows: 1}, false},
		"max joins":            {Policy{MaxJoins: 1}, false},
		"require index":        {Policy{RequireIndexUsage: true}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.policy.IsZero())
		})
	}
}

// Allowed table names are normalized: postgres reports unquoted identifiers in
// lower case, and operators may configure them with padding or mixed case.
func TestPolicyAllowSet(t *testing.T) {
	t.Parallel()

	require.Nil(t, Policy{}.allowSet())

	set := Policy{AllowTables: []string{" Countries ", "LOOKUP", "", "  "}}.allowSet()
	require.Len(t, set, 2)
	require.Contains(t, set, "countries")
	require.Contains(t, set, "lookup")
}
