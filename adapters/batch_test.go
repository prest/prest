package adapters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBoundValues(t *testing.T) {
	t.Parallel()

	require.Nil(t, BoundValues(nil))
	require.False(t, HasDefaults(nil))

	plain := []interface{}{1, "a"}
	require.Equal(t, plain, BoundValues(plain))
	require.False(t, HasDefaults(plain))

	mixed := []interface{}{1, DefaultValue{}, DefaultValue{}, 2}
	require.True(t, HasDefaults(mixed))
	require.Equal(t, []interface{}{1, 2}, BoundValues(mixed))

	all := []interface{}{DefaultValue{}, DefaultValue{}}
	require.Empty(t, BoundValues(all))
}
