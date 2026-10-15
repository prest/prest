package mysql

import (
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

// TestAllowsSchema pins {schema} to the routed database: the registry entry's
// physical database in registry mode, the {database} itself otherwise.
func TestAllowsSchema(t *testing.T) {
	t.Parallel()
	reg := testCfg()
	reg.PGDatabase = "other"
	reg.Databases = []config.DatabaseConf{{Alias: "archive", Database: "other"}, {Alias: "store", Database: "shop"}}
	registry := New(reg).(*Adapter)
	registry.SetDatabase("archive")

	plain := testCfg()
	plain.AllowedDatabases = []string{"other"}
	noRegistry := New(plain).(*Adapter)

	cases := []struct {
		name             string
		scoper           adapters.DatabaseSchemaScoper
		database, schema string
		want             bool
	}{
		{"registry own database", registry, "archive", "other", true},
		{"registry other database", registry, "archive", "shop", false},
		{"registry empty schema", registry, "archive", "", false},
		{"no registry allow-listed", noRegistry, "other", "other", true},
		{"no registry cross database", noRegistry, "shop", "other", false},
		{"no registry default", noRegistry, "shop", "shop", true},
		{"nil cfg same", &Adapter{}, "any", "any", true},
		{"nil cfg cross", &Adapter{}, "any", "other", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.scoper.AllowsDatabaseSchema(tc.database, tc.schema))
		})
	}
}
