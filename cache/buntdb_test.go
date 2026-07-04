package cache

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuntGetDoesntExist(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()

	cache := testConfig().BuntGet("test", w)
	require.False(t, cache)
}
