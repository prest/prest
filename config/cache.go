package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/prest/prest/v2/cache"
	"github.com/spf13/viper"
)

const defaultCacheStoragePath = "./"

func setCacheDefaults(v *viper.Viper) {
	v.SetDefault("cache.enabled", false)
	v.SetDefault("cache.time", 10)
	v.SetDefault("cache.storagepath", defaultCacheStoragePath)
	v.SetDefault("cache.sufixfile", ".cache.prestd.db")
}

// ensureDir ensures path exists as a writable directory.
// It creates missing directories, rejects non-directory paths, and verifies
// writability with a temporary test file.
func ensureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(path, 0700); err != nil {
				return fmt.Errorf("create directory %q: %w", path, err)
			}
		} else {
			return err
		}
	} else if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", path)
	}

	testFile := filepath.Join(path, ".prest-write-test")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		return fmt.Errorf("directory %q is not writable: %w", path, err)
	}
	if err := os.Remove(testFile); err != nil {
		return fmt.Errorf("directory %q is not writable: %w", path, err)
	}
	return nil
}

// ensureCacheStorage ensures the cache storage directory exists and is writable.
// On failure it tries the default path, then disables cache.
func ensureCacheStorage(cfg *Prest) {
	configuredPath := cfg.Cache.StoragePath
	err := ensureDir(configuredPath)
	if err == nil {
		return
	}

	slog.Warn("cache storage path unavailable, trying fallback", "path", configuredPath, "err", err)

	if configuredPath == defaultCacheStoragePath {
		slog.Warn("cache disabled: default storage path unavailable", "path", configuredPath, "err", err)
		cfg.Cache.Enabled = false
		return
	}

	if err = ensureDir(defaultCacheStoragePath); err == nil {
		cfg.Cache.StoragePath = defaultCacheStoragePath
		return
	}

	slog.Warn("cache disabled: fallback storage path unavailable", "path", defaultCacheStoragePath, "err", err)
	cfg.Cache.Enabled = false
}

func loadCacheConfig(v *viper.Viper, cfg *Prest) {
	cfg.Cache.Enabled = v.GetBool("cache.enabled")
	cfg.Cache.Time = v.GetInt("cache.time")
	cfg.Cache.StoragePath = v.GetString("cache.storagepath")
	cfg.Cache.SufixFile = v.GetString("cache.sufixfile")

	cfg.Cache.Endpoints = unmarshalKeyOrZero[[]cache.Endpoint](v, "cache.endpoints")
}
