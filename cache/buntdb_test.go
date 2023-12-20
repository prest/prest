package cache

import (
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/config"
	"github.com/stretchr/testify/require"
)

func TestBuntGetDoesntExist(t *testing.T) {
	config.PrestConf = &config.Prest{
		Cache: config.Cache{
			Enabled:     true,
			Time:        10,
			Endpoints:   []config.CacheEndpoint{},
			StoragePath: "./",
		},
	}

	w := httptest.NewRecorder()

	cache := BuntGet("test", w)
	require.False(t, cache)
}
