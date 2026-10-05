package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prest/prest/v2/cache"
	"github.com/stretchr/testify/require"
)

func TestEnsureDir(t *testing.T) {
	t.Run("creates missing directory", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "nested", "queries")
		require.NoError(t, ensureDir(path))
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.True(t, info.IsDir())
	})

	t.Run("existing writable directory", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, ensureDir(t.TempDir()))
	})

	t.Run("path is not a directory", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(path, []byte("x"), 0600))
		err := ensureDir(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not a directory")
	})

	t.Run("cannot create directory in read-only parent", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		parent := filepath.Join(base, "readonly-parent")
		require.NoError(t, os.Mkdir(parent, 0500))
		err := ensureDir(filepath.Join(parent, "child"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "create directory")
	})

	t.Run("directory not writable", func(t *testing.T) {
		path := t.TempDir()
		require.NoError(t, os.Chmod(path, 0000))
		t.Cleanup(func() { _ = os.Chmod(path, 0700) })
		err := ensureDir(path)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not writable")
	})
}

func TestEnsureCacheStorage(t *testing.T) {
	t.Run("ok configured path", func(t *testing.T) {
		t.Parallel()
		storagePath := t.TempDir()
		cfg := &Prest{Cache: cache.Config{Enabled: true, StoragePath: storagePath}}
		ensureCacheStorage(cfg)
		require.True(t, cfg.Cache.Enabled)
		require.Equal(t, storagePath, cfg.Cache.StoragePath)
	})

	t.Run("falls back to default path", func(t *testing.T) {
		t.Chdir(t.TempDir())
		cfg := &Prest{Cache: cache.Config{Enabled: true, StoragePath: inaccessiblePath(t)}}
		ensureCacheStorage(cfg)
		require.True(t, cfg.Cache.Enabled)
		require.Equal(t, defaultCacheStoragePath, cfg.Cache.StoragePath)
	})

	t.Run("disables cache when default storage path unavailable", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		require.NoError(t, os.Chmod(tempDir, 0000))
		t.Cleanup(func() { _ = os.Chmod(tempDir, 0700) })

		cfg := &Prest{Cache: cache.Config{Enabled: true, StoragePath: defaultCacheStoragePath}}
		ensureCacheStorage(cfg)
		require.False(t, cfg.Cache.Enabled)
	})

	t.Run("disables cache when configured and fallback paths fail", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Chdir(tempDir)
		require.NoError(t, os.Chmod(tempDir, 0000))
		t.Cleanup(func() { _ = os.Chmod(tempDir, 0700) })

		cfg := &Prest{Cache: cache.Config{Enabled: true, StoragePath: inaccessiblePath(t)}}
		ensureCacheStorage(cfg)
		require.False(t, cfg.Cache.Enabled)
	})
}

func inaccessiblePath(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	restricted := filepath.Join(base, "restricted")
	require.NoError(t, os.Mkdir(restricted, 0700))
	target := filepath.Join(restricted, "target")
	require.NoError(t, os.Chmod(restricted, 0000))
	t.Cleanup(func() { _ = os.Chmod(restricted, 0700) })
	return target
}
