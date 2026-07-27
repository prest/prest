package queryguard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A policy set is empty only when neither the default nor any override enforces
// a rule; a single enforcing override is enough to require guarding.
func TestPoliciesIsZero(t *testing.T) {
	t.Parallel()

	require.True(t, Policies{}.IsZero())
	require.True(t, Policies{Databases: map[string]Policy{"a": {}}}.IsZero())
	require.False(t, Policies{Default: Policy{MaxCost: 1}}.IsZero())
	require.False(t, Policies{Databases: map[string]Policy{"a": {RejectSeqScan: true}}}.IsZero())
}

// Overrides are matched case-insensitively and fall back to the default.
func TestPoliciesFor(t *testing.T) {
	t.Parallel()

	policies := Policies{
		Default:   Policy{MaxCost: 10},
		Databases: map[string]Policy{"analytics": {MaxCost: 99}},
	}

	require.Equal(t, float64(99), policies.For("Analytics").MaxCost)
	require.Equal(t, float64(10), policies.For("billing").MaxCost)
	require.Equal(t, float64(10), policies.For("").MaxCost)
	require.Equal(t, float64(10), Policies{Default: Policy{MaxCost: 10}}.For("analytics").MaxCost)
}

// Configured keys are matched case-insensitively too: Policies is built straight
// from code in tests and plugins, not only from the lower-cased config keys.
func TestPoliciesForMixedCaseKey(t *testing.T) {
	t.Parallel()

	policies := Policies{
		Default:   Policy{MaxCost: 10},
		Databases: map[string]Policy{"Analytics": {MaxCost: 99}},
	}

	require.Equal(t, float64(99), policies.For("analytics").MaxCost)
	require.Equal(t, float64(99), policies.For("ANALYTICS").MaxCost)
	require.Equal(t, float64(99), policies.For("Analytics").MaxCost)
	require.Equal(t, float64(10), policies.For("billing").MaxCost)
}
