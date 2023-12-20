package cache

import (
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/config"

	"github.com/stretchr/testify/require"
)

func TestBuntGetDoesntExist(t *testing.T) {
	t.Setenv("PREST_CACHE", "true")
	config.Load()
	w := httptest.NewRecorder()

	cache := BuntGet("test", w)
	require.False(t, cache)
}
