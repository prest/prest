package cache

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/avelino/slugify"
	"github.com/stretchr/testify/require"
)

func TestBuntConnectOpensWithTempDir(t *testing.T) {
	t.Parallel()

	cfg := buntCacheConfig(t)

	db, err := cfg.BuntConnect("")
	require.NoError(t, err)
	require.NotNil(t, db)
	require.True(t, cfg.Enabled)
	require.NoError(t, db.Close())

	_, err = os.Stat(filepath.Join(cfg.StoragePath, cfg.SufixFile))
	require.NoError(t, err)
}

func TestBuntConnectSlugifiesKey(t *testing.T) {
	t.Parallel()

	cfg := buntCacheConfig(t)
	key := "http://example.com/foo bar"

	db, err := cfg.BuntConnect(key)
	require.NoError(t, err)
	require.NotNil(t, db)
	require.NoError(t, db.Close())

	wantFile := slugify.Slugify(key) + cfg.SufixFile
	_, err = os.Stat(filepath.Join(cfg.StoragePath, wantFile))
	require.NoError(t, err)
}

func TestBuntConnectOpenErrorDisablesCache(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	cfg := &Config{
		Enabled:     true,
		StoragePath: blocker,
		SufixFile:   ".cache.prestd.db",
	}

	db, err := cfg.BuntConnect("")
	require.Error(t, err)
	require.Nil(t, db)
	require.False(t, cfg.Enabled)
}

func TestBuntGetMiss(t *testing.T) {
	t.Parallel()

	cfg := buntCacheConfig(t)
	w := httptest.NewRecorder()

	found := cfg.BuntGet("missing-key", w)
	require.False(t, found)
	require.Equal(t, 200, w.Code)
	require.Empty(t, w.Header().Get("Cache-Server"))
	require.Empty(t, w.Body.String())
}

func TestBuntGetHit(t *testing.T) {
	t.Parallel()

	const key = "/prest/public/test"
	cfg := buntCacheConfig(t)
	cfg.BuntSet(key, `[{"cached":true}]`)

	w := httptest.NewRecorder()
	found := cfg.BuntGet(key, w)

	require.True(t, found)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "prestd", w.Header().Get("Cache-Server"))
	require.JSONEq(t, `[{"cached":true}]`, w.Body.String())
}

func TestBuntGetConnectError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	cfg := &Config{
		Enabled:     true,
		StoragePath: blocker,
		SufixFile:   ".cache.prestd.db",
	}
	w := httptest.NewRecorder()

	found := cfg.BuntGet("any-key", w)
	require.False(t, found)
	require.Empty(t, w.Header().Get("Cache-Server"))
}

func TestBuntSetDisabled(t *testing.T) {
	t.Parallel()

	const key = "/prest/public/test"
	cfg := buntCacheConfig(t)
	cfg.Enabled = false

	cfg.BuntSet(key, "should-not-cache")

	w := httptest.NewRecorder()
	require.False(t, cfg.BuntGet(key, w))
}

func TestBuntSetEndpointRulesSkip(t *testing.T) {
	t.Parallel()

	const key = "/prest/public/test"
	cfg := buntCacheConfig(t)
	cfg.Endpoints = []Endpoint{
		{Enabled: true, Endpoint: "/other", Time: 5},
	}

	cfg.BuntSet(key, "should-not-cache")

	w := httptest.NewRecorder()
	require.False(t, cfg.BuntGet(key, w))
}

func TestBuntSetSuccessWithEndpointRule(t *testing.T) {
	t.Parallel()

	const path = "/prest/public/test"
	const key = path + "?limit=10"
	cfg := buntCacheConfig(t)
	cfg.Endpoints = []Endpoint{
		{Enabled: true, Endpoint: path, Time: 5},
	}

	cfg.BuntSet(key, "cached-value")

	w := httptest.NewRecorder()
	found := cfg.BuntGet(key, w)
	require.True(t, found)
	require.Equal(t, "cached-value", w.Body.String())
}

func TestBuntSetSuccessWithDefaultEndpointRules(t *testing.T) {
	t.Parallel()

	const key = "/prest/public/default"
	cfg := buntCacheConfig(t)

	cfg.BuntSet(key, "default-rule-value")

	w := httptest.NewRecorder()
	found := cfg.BuntGet(key, w)
	require.True(t, found)
	require.Equal(t, "default-rule-value", w.Body.String())
}

func TestBuntSetConnectError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	cfg := &Config{
		Enabled:     true,
		Time:        10,
		StoragePath: blocker,
		SufixFile:   ".cache.prestd.db",
		Endpoints:   []Endpoint{},
	}

	require.NotPanics(t, func() {
		cfg.BuntSet("/prest/public/test", "value")
	})
}
