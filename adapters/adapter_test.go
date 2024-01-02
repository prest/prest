package adapters

import (
	"fmt"
	"testing"

	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/config"
	"github.com/stretchr/testify/require"
)

func Test_New(t *testing.T) {
	cfg := &config.Prest{Adapter: "postgres"}
	adapter, err := New(cfg)
	require.NoError(t, err)
	require.IsType(t, &postgres.Adapter{}, adapter)

	cfg.Adapter = ""
	adapter, err = New(cfg)
	require.NoError(t, err)
	require.IsType(t, &postgres.Adapter{}, adapter)

	cfg.Adapter = "invalid"
	adapter, err = New(cfg)
	require.Error(t, err)
	require.Nil(t, adapter)
	expectedErr := fmt.Errorf("adapter '%s' not supported", cfg.Adapter)
	require.EqualError(t, err, expectedErr.Error())
}
