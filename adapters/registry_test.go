package adapters_test

import (
	"testing"

	"github.com/prest/prest/v2/adapters"

	"github.com/stretchr/testify/require"
)

// stubAdapter satisfies Adapter for registry bookkeeping; no method is called.
type stubAdapter struct {
	adapters.Adapter
	name string
}

// TestSimpleRegistry_GetAllSorted pins alphabetical order: callers such as
// startup migration and the default-adapter choice must not depend on map
// iteration order.
func TestSimpleRegistry_GetAllSorted(t *testing.T) {
	t.Parallel()

	r := adapters.NewRegistry()
	for _, alias := range []string{"delta", "alpha", "charlie", "bravo"} {
		require.NoError(t, r.Register(alias, &stubAdapter{}))
	}
	want := []string{"alpha", "bravo", "charlie", "delta"}
	for range 20 {
		require.Equal(t, want, r.GetAll())
	}
	require.Equal(t, want, r.Aliases())
}

// TestSimpleRegistry_RegisterGet covers lookup, re-registration and the
// unknown-alias sentinel.
func TestSimpleRegistry_RegisterGet(t *testing.T) {
	t.Parallel()

	r := adapters.NewRegistry()
	first, second := &stubAdapter{name: "first"}, &stubAdapter{name: "second"}
	require.NoError(t, r.Register("a", first))
	require.True(t, r.IsRegistered("a"))
	require.False(t, r.IsRegistered("b"))

	got, err := r.Get("a")
	require.NoError(t, err)
	require.Same(t, first, got)

	require.NoError(t, r.Register("a", second))
	got, err = r.Get("a")
	require.NoError(t, err)
	require.Same(t, second, got)

	_, err = r.Get("b")
	require.ErrorIs(t, err, adapters.ErrAdapterNotFound)
}

// TestSimpleRegistry_RegisterRejectsInvalid covers the empty alias and nil
// adapter guards.
func TestSimpleRegistry_RegisterRejectsInvalid(t *testing.T) {
	t.Parallel()

	r := adapters.NewRegistry()
	require.Error(t, r.Register("", &stubAdapter{}))
	require.Error(t, r.Register("a", nil))
	require.Empty(t, r.GetAll())
}
