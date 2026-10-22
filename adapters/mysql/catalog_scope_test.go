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
		{Alias: "a", Engine: config.EngineMySQL, Host: "db.internal", Port: 3306, Database: "app_a"},
		{Alias: "b", Host: "db.internal", Port: 3306, Database: "app_b"},          // engine-less: inherits mysql
		{Alias: "c", Engine: config.EngineMySQL, Host: "db.internal", Port: 3306}, // no database: runs as its alias
	}
	q, count := New(cfg).(*Adapter).SchemaClause(req("/schemas"))
	require.False(t, count)
	require.Contains(t, q, "AND SCHEMA_NAME IN ('app_a', 'app_b', 'c')\n) s")
	require.NotContains(t, q, "tenant_other")

	q, count = New(cfg).(*Adapter).SchemaClause(req("/schemas?_count=schema_name"))
	require.True(t, count)
	require.Contains(t, q, "COUNT(schema_name)")
	require.Contains(t, q, "SCHEMA_NAME IN ('app_a', 'app_b', 'c')")
}

// TestSchemaClause_RegistryIgnoresOtherTargets: a registry mixes engines and
// servers, so only entries on this adapter's own MySQL connection may widen
// its /schemas allow-list; a same-named database elsewhere is not served here.
func TestSchemaClause_RegistryIgnoresOtherTargets(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.Databases = []config.DatabaseConf{
		{Alias: "self", Engine: config.EngineMySQL, Host: "db.internal", Port: 3306, Database: "shop"},
		{Alias: "pg", Engine: config.EnginePostgres, Host: "db.internal", Port: 3306, Database: "pg_only"},
		{Alias: "otherhost", Engine: config.EngineMySQL, Host: "tenant.internal", Port: 3306, Database: "tenant_other"},
		{Alias: "otherport", Engine: config.EngineMySQL, Host: "db.internal", Port: 3307, Database: "port_other"},
		{Alias: "nodb", Engine: config.EngineMySQL, Host: "tenant.internal", Port: 3306}, // must not fall back to pg.database
	}
	q, _ := New(cfg).(*Adapter).SchemaClause(req("/schemas"))
	require.Contains(t, q, "AND SCHEMA_NAME IN ('shop')\n) s")
	for _, leaked := range []string{"pg_only", "tenant_other", "port_other", "nodb"} {
		require.NotContains(t, q, leaked)
	}
}

// TestSchemaClause_RegistryFilterIsInsideDerivedTable: the controller appends
// " WHERE <request>" after the clause, so the served filter must live inside
// the derived table where a request filter cannot reach it.
func TestSchemaClause_RegistryFilterIsInsideDerivedTable(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.Databases = []config.DatabaseConf{{Alias: "a", Host: "db.internal", Port: 3306, Database: "app_a"}}
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
	cfg := &config.Prest{Engine: config.EngineMySQL, PGHost: "h", PGPort: 3306, PGDatabase: "def", Databases: []config.DatabaseConf{
		{Alias: "x", Host: "h", Port: 3306, Database: "d"},
		{Alias: "y", Host: "H", Port: 3306, Database: "d"}, // host names compare case-insensitively
		{Alias: "z", Host: "h", Port: 3306},                // resolves to its alias, never pg.database
		{Alias: "w", Host: "elsewhere", Port: 3306, Database: "e"},
	}}
	require.Equal(t, []string{"d", "z"}, (&Adapter{cfg: cfg}).registryPhysicalNames())
	// No entry on this connection: an empty, non-nil list, so the filter fails closed.
	cfg = &config.Prest{Engine: config.EngineMySQL, PGHost: "h", PGPort: 3306, Databases: []config.DatabaseConf{{Alias: "x", Host: "other", Port: 3306}}}
	require.Equal(t, []string{}, (&Adapter{cfg: cfg}).registryPhysicalNames())
}
