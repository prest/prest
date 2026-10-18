package mysql

import (
	"testing"

	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

// TestSchemaClause_RegistryListsOnlyRegisteredDatabases: GHSA-fhf5-m388-4w4p,
// /schemas in registry mode must not list databases (other tenants) that no
// registry entry serves.
func TestSchemaClause_RegistryListsOnlyRegisteredDatabases(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.Databases = []config.DatabaseConf{
		{Alias: "a", Database: "app_a"},
		{Alias: "b", Database: "app_b"},
		{Alias: "c"}, // no database: falls back to pg.database ("shop")
	}
	q, count := New(cfg).(*Adapter).SchemaClause(req("/schemas"))
	require.False(t, count)
	require.Contains(t, q, "AND SCHEMA_NAME IN ('app_a', 'app_b', 'shop')\n) s")
	require.NotContains(t, q, "tenant_other")

	q, count = New(cfg).(*Adapter).SchemaClause(req("/schemas?_count=schema_name"))
	require.True(t, count)
	require.Contains(t, q, "COUNT(schema_name)")
	require.Contains(t, q, "SCHEMA_NAME IN ('app_a', 'app_b', 'shop')")
}

// TestSchemaClause_RegistryFilterIsInsideDerivedTable: the controller appends
// " WHERE <request>" after the clause, so the served filter must live inside
// the derived table where a request filter cannot reach it.
func TestSchemaClause_RegistryFilterIsInsideDerivedTable(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.Databases = []config.DatabaseConf{{Alias: "a", Database: "app_a"}}
	q, _ := New(cfg).(*Adapter).SchemaClause(req("/schemas?schema_name=victim"))
	require.Regexp(t, `(?s)WHERE SCHEMA_NAME NOT IN \(.*\) AND SCHEMA_NAME IN \('app_a'\)\n\) s$`, q)
}

func TestSchemaClause_NonRegistryUnchanged(t *testing.T) {
	t.Parallel()
	q, _ := New(testCfg()).(*Adapter).SchemaClause(req("/schemas"))
	require.Contains(t, q, "AND SCHEMA_NAME IN ('shop')\n) s")
}

func TestRegistryPhysicalNames(t *testing.T) {
	t.Parallel()
	require.Nil(t, (&Adapter{}).registryPhysicalNames())
	cfg := &config.Prest{PGDatabase: "def", Databases: []config.DatabaseConf{{Alias: "x", Database: "d"}, {Alias: "y", Database: "d"}, {Alias: "z"}}}
	require.Equal(t, []string{"d", "def"}, (&Adapter{cfg: cfg}).registryPhysicalNames())
	cfg = &config.Prest{Databases: []config.DatabaseConf{{Alias: "x"}}}
	require.Equal(t, []string{"x"}, (&Adapter{cfg: cfg}).registryPhysicalNames())
}
