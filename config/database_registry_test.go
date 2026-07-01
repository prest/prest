package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func resetViperForTest(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestParseDatabaseRegistry_EnvIndexed(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "PREST_DATABASES")
	unsetEnvForTest(t, "DATABASE_ALIAS_1")
	unsetEnvForTest(t, "DATABASE_URL_1")
	unsetEnvForTest(t, "DATABASE_ALIAS_2")
	unsetEnvForTest(t, "DATABASE_URL_2")

	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	t.Setenv("DATABASE_URL_1", "postgres://user:pass@cluster-a.example.com:5432/app_a?sslmode=require")
	t.Setenv("DATABASE_ALIAS_2", "tenant-b")
	t.Setenv("DATABASE_URL_2", "postgres://user:pass@cluster-b.example.com:5432/app_b?sslmode=disable")

	cfg := &Prest{}
	parseDBConfig(cfg)
	err := parseDatabaseRegistry(cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Databases, 2)
	require.Equal(t, "tenant-a", cfg.Databases[0].Alias)
	require.Equal(t, "app_a", cfg.Databases[0].Database)
	require.Equal(t, "cluster-a.example.com", cfg.Databases[0].Host)
	require.Equal(t, "require", cfg.Databases[0].SSL.Mode)
}

func TestParseDatabaseRegistry_ManifestEnv(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "DATABASE_ALIAS_1")
	unsetEnvForTest(t, "DATABASE_URL_1")
	t.Setenv("PREST_DATABASES", "tenant-a,tenant-b")
	t.Setenv("PREST_DATABASE_TENANT_A_URL", "postgres://user:pass@host-a:5432/app_a?sslmode=disable")
	t.Setenv("PREST_DATABASE_TENANT_B_URL", "postgres://user:pass@host-b:5432/app_b?sslmode=disable")

	cfg := &Prest{}
	parseDBConfig(cfg)
	err := parseDatabaseRegistry(cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Databases, 2)
	require.Equal(t, "tenant-a", cfg.Databases[0].Alias)
	require.Equal(t, "app_b", cfg.Databases[1].Database)
}

func TestParseDatabaseRegistry_EnvOverridesTOML(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "DATABASE_ALIAS_1")
	unsetEnvForTest(t, "DATABASE_URL_1")
	unsetEnvForTest(t, "PREST_DATABASES")

	t.Setenv("PREST_CONF", "../testdata/databases.toml")
	unsetEnvForTest(t, "PREST_DATABASES")

	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	t.Setenv("DATABASE_URL_1", "postgres://override:override@override-host:5432/override_db?sslmode=require")

	viperCfg()
	require.NoError(t, viper.ReadInConfig())
	cfg := &Prest{}
	parseDBConfig(cfg)
	err := parseDatabaseRegistry(cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Databases, 2)
	require.Equal(t, "override-host", cfg.Databases[0].Host)
	require.Equal(t, "override_db", cfg.Databases[0].Database)
	require.Equal(t, "tenant-b", cfg.Databases[1].Alias)
}

func TestParseDatabaseRegistry_LegacyUnchanged(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "DATABASE_ALIAS_1")
	unsetEnvForTest(t, "DATABASE_URL_1")
	unsetEnvForTest(t, "PREST_DATABASES")
	unsetEnvForTest(t, "DATABASE_URL")

	t.Setenv("DATABASE_URL", "postgresql://cloud:cloudPass@localhost:5432/CloudDatabase/?sslmode=disable")
	cfg := &Prest{}
	parseDBConfig(cfg)
	err := parseDatabaseRegistry(cfg)
	require.NoError(t, err)
	require.Empty(t, cfg.Databases)
	require.Equal(t, "CloudDatabase", cfg.PGDatabase)
}

func TestParseDatabaseRegistry_MissingURL(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "PREST_DATABASES")
	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	unsetEnvForTest(t, "DATABASE_URL_1")

	cfg := &Prest{}
	parseDBConfig(cfg)
	err := parseDatabaseRegistry(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "DATABASE_URL_1")
}

func TestParseDatabaseRegistry_DuplicateAlias(t *testing.T) {
	resetViperForTest(t)
	unsetEnvForTest(t, "PREST_DATABASES")
	t.Setenv("DATABASE_ALIAS_1", "tenant-a")
	t.Setenv("DATABASE_URL_1", "postgres://user:pass@host:5432/app_a?sslmode=disable")
	t.Setenv("DATABASE_ALIAS_2", "tenant-a")
	t.Setenv("DATABASE_URL_2", "postgres://user:pass@host:5432/app_b?sslmode=disable")

	cfg := &Prest{}
	parseDBConfig(cfg)
	err := parseDatabaseRegistry(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}

func TestHasDatabaseRegistry(t *testing.T) {
	require.False(t, HasDatabaseRegistry(nil))
	require.False(t, HasDatabaseRegistry(&Prest{}))
	require.True(t, HasDatabaseRegistry(&Prest{Databases: []DatabaseConf{{Alias: "a"}}}))
}
