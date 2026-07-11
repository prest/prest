package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestParseQueriesConfig_Defaults(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.SetDefault("queries.storage", QueriesStorageFilesystem)
	cfg := &Prest{}
	parseQueriesConfig(v, cfg)

	require.Equal(t, QueriesStorageFilesystem, cfg.QueriesConf.Storage)
	require.Equal(t, "public", cfg.QueriesConf.Schema)
	require.Equal(t, "prest_queries", cfg.QueriesConf.Table)
	require.False(t, cfg.QueriesConf.Restrict)
	require.False(t, cfg.QueriesConf.RegisterEnabled)
	require.Equal(t, QueriesImportPolicyUpdate, cfg.QueriesConf.ImportPolicy)
	require.False(t, cfg.QueriesConf.ImportOnStartup)
	require.False(t, cfg.QueriesConf.MigrateOnStartup)
}

func TestParseQueriesConfig_DatabaseImportDefault(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("queries.storage", QueriesStorageDatabase)
	cfg := &Prest{}
	parseQueriesConfig(v, cfg)

	require.Equal(t, QueriesStorageDatabase, cfg.QueriesConf.Storage)
	require.True(t, cfg.QueriesConf.ImportOnStartup)
	require.True(t, cfg.QueriesConf.MigrateOnStartup)
}

func TestParseQueriesConfig_ExplicitMigrateOnStartup(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("queries.storage", QueriesStorageDatabase)
	v.Set("queries.migrate_on_startup", false)
	cfg := &Prest{}
	parseQueriesConfig(v, cfg)

	require.False(t, cfg.QueriesConf.MigrateOnStartup)
	require.True(t, cfg.QueriesConf.ImportOnStartup)
}

func TestEnsureQueriesConfig_RegisterDisabledWithoutAuth(t *testing.T) {
	t.Parallel()

	cfg := &Prest{
		AuthEnabled: false,
		QueriesConf: QueriesConf{
			RegisterEnabled: true,
			RegisterAdmins:  []string{"admin"},
		},
	}
	ensureQueriesConfig(cfg)
	require.False(t, cfg.QueriesConf.RegisterEnabled)
}

func TestEnsureQueriesConfig_RestrictDisabledWithoutAuth(t *testing.T) {
	t.Parallel()

	cfg := &Prest{
		AuthEnabled: false,
		QueriesConf: QueriesConf{Restrict: true},
	}
	ensureQueriesConfig(cfg)
	require.False(t, cfg.QueriesConf.Restrict)
}

func TestEnsureQueriesConfig_InvalidImportPolicy(t *testing.T) {
	t.Parallel()

	cfg := &Prest{
		QueriesConf: QueriesConf{ImportPolicy: "nope"},
	}
	ensureQueriesConfig(cfg)
	require.Equal(t, QueriesImportPolicyUpdate, cfg.QueriesConf.ImportPolicy)
}
