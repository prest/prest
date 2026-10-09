package mysql

import (
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestAllowsSchemaWithoutRegistry(t *testing.T) {
	t.Parallel()
	a := New(testCfg()).(*Adapter)
	require.True(t, a.AllowsSchema("shop"))
	require.True(t, a.AllowsSchema("other"))
	require.True(t, (&Adapter{}).AllowsSchema("any"))
}

func TestAllowsSchemaPinnedToPhysicalDatabase(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.PGDatabase = "other"
	cfg.Databases = []config.DatabaseConf{{Alias: "main", Database: "shop"}, {Alias: "second", Database: "other"}}
	var scoper adapters.SchemaScoper = New(cfg).(*Adapter)
	require.True(t, scoper.AllowsSchema("other"))
	require.False(t, scoper.AllowsSchema("shop"))
	require.False(t, scoper.AllowsSchema(""))
}
