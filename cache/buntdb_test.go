package cache

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuntGetDoesntExist(t *testing.T) {
	c := Config{
		Enabled:     true,
		Time:        10,
		Endpoints:   []Endpoint{},
		StoragePath: "./",
	}

	w := httptest.NewRecorder()

	cache := c.BuntGet("test", w)
	require.False(t, cache)
}
