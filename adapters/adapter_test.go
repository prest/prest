package adapters

import (
	"testing"

	"github.com/prest/prest/config"
	"github.com/stretchr/testify/require"
)

func Test_New(t *testing.T) {
	cfg := &config.Prest{}
	cfg.Adapter = ""
	adapter, err := New(cfg)
	require.NoError(t, err)

	_, ok := adapter.(Adapter)
	require.True(t, ok)

	cfg.Adapter = "invalid"
	adapter, err = New(cfg)
	require.Error(t, err)
	require.Nil(t, adapter)

	require.Equal(t, err, ErrAdapterNotSupported)
}
