package cache

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuntGetDoesntExist(t *testing.T) {
	w := httptest.NewRecorder()

	cache := BuntGet("test", w)
	require.False(t, cache)
}
