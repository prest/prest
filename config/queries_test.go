package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// homeEnvMu serializes tests that mutate HOME/USERPROFILE because defaultQueriesPath
// reads homedir at runtime and parallel tests would race on the process environment.
var homeEnvMu sync.Mutex

func withIsolatedHome(t *testing.T, home string) {
	t.Helper()
	homeEnvMu.Lock()
	t.Cleanup(func() {
		homedir.Reset()
		homeEnvMu.Unlock()
	})
	homedir.Reset()
	t.Setenv("HOME", home)
}

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

func TestParseQueriesConfig_ScriptsAndUsers(t *testing.T) {
	t.Parallel()

	t.Run("valid structured keys", func(t *testing.T) {
		t.Parallel()

		v := viper.New()
		v.Set("queries.scripts", []map[string]any{
			{"name": "list_users", "permissions": []string{"read"}},
		})
		v.Set("queries.users", []map[string]any{
			{"name": "alice", "scripts": []map[string]any{
				{"name": "list_users", "permissions": []string{"read"}},
			}},
		})
		cfg := &Prest{}
		parseQueriesConfig(v, cfg)

		require.Len(t, cfg.QueriesConf.Scripts, 1)
		require.Equal(t, "list_users", cfg.QueriesConf.Scripts[0].Name)
		require.Equal(t, []string{"read"}, cfg.QueriesConf.Scripts[0].Permissions)
		require.Len(t, cfg.QueriesConf.Users, 1)
		require.Equal(t, "alice", cfg.QueriesConf.Users[0].Name)
		require.Len(t, cfg.QueriesConf.Users[0].Scripts, 1)
	})

	t.Run("invalid keys fall back to zero value", func(t *testing.T) {
		t.Parallel()

		v := viper.New()
		v.Set("queries.scripts", "not-an-array")
		v.Set("queries.users", 123)
		cfg := &Prest{}
		parseQueriesConfig(v, cfg)

		require.Empty(t, cfg.QueriesConf.Scripts)
		require.Empty(t, cfg.QueriesConf.Users)
	})
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

func TestEnsureQueriesPath_DatabaseModeSkipsWhenImportDisabled(t *testing.T) {
	t.Parallel()

	importPath := filepath.Join(t.TempDir(), "does-not-exist-yet")
	cfg := &Prest{
		QueriesPath: importPath,
		QueriesConf: QueriesConf{
			Storage:         QueriesStorageDatabase,
			ImportOnStartup: false,
		},
	}
	ensureQueriesPath(cfg)

	_, err := os.Stat(importPath)
	require.True(t, os.IsNotExist(err))
	require.Equal(t, importPath, cfg.QueriesPath)
}

func TestEnsureQueriesPath_DatabaseModeProvisionsWhenImportEnabled(t *testing.T) {
	t.Parallel()

	importPath := filepath.Join(t.TempDir(), "import-queries")
	cfg := &Prest{
		QueriesPath: importPath,
		QueriesConf: QueriesConf{
			Storage:         QueriesStorageDatabase,
			ImportOnStartup: true,
		},
	}
	ensureQueriesPath(cfg)

	info, err := os.Stat(importPath)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestDefaultQueriesPath(t *testing.T) {
	t.Run("fallback when homedir unavailable", func(t *testing.T) {
		homeEnvMu.Lock()
		t.Cleanup(func() {
			homedir.Reset()
			homeEnvMu.Unlock()
		})
		homedir.Reset()
		unsetEnvForTest(t, "HOME")
		unsetEnvForTest(t, "USERPROFILE")
		unsetEnvForTest(t, "HOMEDRIVE")
		unsetEnvForTest(t, "HOMEPATH")
		// On macOS, unset HOME still resolves via dscl; empty PATH blocks OS fallbacks.
		unsetEnvForTest(t, "PATH")
		_, err := homedir.Dir()
		require.Error(t, err, "test setup must prevent homedir resolution")
		require.Equal(t, filepath.Join(".", "queries"), defaultQueriesPath())
	})

	t.Run("uses home directory", func(t *testing.T) {
		withIsolatedHome(t, t.TempDir())
		require.Equal(t, filepath.Join(os.Getenv("HOME"), "queries"), defaultQueriesPath())
	})
}

func TestEnsureQueriesPath_FilesystemModeUsesFallbackWhenConfiguredPathFails(t *testing.T) {
	home := t.TempDir()
	withIsolatedHome(t, home)
	cfg := &Prest{
		QueriesPath: inaccessiblePath(t),
		QueriesConf: QueriesConf{Storage: QueriesStorageFilesystem},
	}
	ensureQueriesPath(cfg)
	require.Equal(t, filepath.Join(home, "queries"), cfg.QueriesPath)
}

func TestEnsureQueriesPath_FilesystemModeClearsPathWhenFallbackFails(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.Chmod(home, 0000))
	t.Cleanup(func() { _ = os.Chmod(home, 0700) })
	withIsolatedHome(t, home)

	cfg := &Prest{
		QueriesPath: filepath.Join(home, "queries"),
		QueriesConf: QueriesConf{Storage: QueriesStorageFilesystem},
	}
	ensureQueriesPath(cfg)
	require.Empty(t, cfg.QueriesPath)
}

func TestEnsureQueriesPath_DatabaseModeSkipsEmptyImportPath(t *testing.T) {
	t.Parallel()
	cfg := &Prest{
		QueriesConf: QueriesConf{
			Storage:         QueriesStorageDatabase,
			ImportOnStartup: true,
		},
	}
	ensureQueriesPath(cfg)
	require.Empty(t, cfg.QueriesPath)
}

func TestEnsureQueriesPath_DatabaseModeWarnsWhenImportPathUnavailable(t *testing.T) {
	importPath := inaccessiblePath(t)
	cfg := &Prest{
		QueriesPath: importPath,
		QueriesConf: QueriesConf{
			Storage:         QueriesStorageDatabase,
			ImportOnStartup: true,
		},
	}
	ensureQueriesPath(cfg)
	require.Equal(t, importPath, cfg.QueriesPath)
}

func TestEnsureQueriesConfig_InvalidImportPolicy(t *testing.T) {
	t.Parallel()

	cfg := &Prest{
		QueriesConf: QueriesConf{ImportPolicy: "nope"},
	}
	ensureQueriesConfig(cfg)
	require.Equal(t, QueriesImportPolicyUpdate, cfg.QueriesConf.ImportPolicy)
}
