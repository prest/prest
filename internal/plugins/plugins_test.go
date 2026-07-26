package plugins

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignPluginHTTPVars(t *testing.T) {
	t.Parallel()

	t.Run("valid symbol", func(t *testing.T) {
		vars := make(map[string]string)
		err := assignPluginHTTPVars(&vars, map[string]string{"file": "hello", "func": "Hello"})
		require.NoError(t, err)
		assert.Equal(t, "hello", vars["file"])
		assert.Equal(t, "Hello", vars["func"])
	})

	t.Run("invalid symbol type", func(t *testing.T) {
		err := assignPluginHTTPVars("not a map pointer", map[string]string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "HTTPVars")
	})
}

func TestAssignPluginURLQuery(t *testing.T) {
	t.Parallel()

	t.Run("valid symbol", func(t *testing.T) {
		query := make(map[string][]string)
		err := assignPluginURLQuery(&query, map[string][]string{"q": {"test"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"test"}, query["q"])
	})

	t.Run("invalid symbol type", func(t *testing.T) {
		err := assignPluginURLQuery(42, map[string][]string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "URLQuery")
	})
}

func TestInvokePluginHandler(t *testing.T) {
	t.Parallel()

	t.Run("func() string handler", func(t *testing.T) {
		ret, err := invokePluginHandler(func() string { return `{"ok":true}` }, "GETHelloHandler")
		require.NoError(t, err)
		assert.Equal(t, `{"ok":true}`, ret.ReturnJson)
		assert.Equal(t, -1, ret.StatusCode)
	})

	t.Run("func() (string, int) handler", func(t *testing.T) {
		ret, err := invokePluginHandler(func() (string, int) { return `{"ok":true}`, http.StatusAccepted }, "GETHelloWithStatusHandler")
		require.NoError(t, err)
		assert.Equal(t, `{"ok":true}`, ret.ReturnJson)
		assert.Equal(t, http.StatusAccepted, ret.StatusCode)
	})

	t.Run("invalid handler type", func(t *testing.T) {
		_, err := invokePluginHandler(func() int { return 0 }, "GETBadHandler")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "GETBadHandler")
	})
}

func TestPluginInvokeMutexReturnsSameMutexForPath(t *testing.T) {
	t.Parallel()

	mu1 := pluginInvokeMutex("/lib/hello.so")
	mu2 := pluginInvokeMutex("/lib/hello.so")
	mu3 := pluginInvokeMutex("/lib/other.so")

	assert.Same(t, mu1, mu2)
	assert.NotSame(t, mu1, mu3)
}

func TestPluginInvokeSerialization(t *testing.T) {
	const libPath = "/test/plugin.so"
	mu := pluginInvokeMutex(libPath)

	var pluginHTTPVars map[string]string
	var violations int
	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()

			want := fmt.Sprintf("request-%d", id)
			err := assignPluginHTTPVars(&pluginHTTPVars, map[string]string{"id": want})
			require.NoError(t, err)

			time.Sleep(time.Millisecond)
			if pluginHTTPVars["id"] != want {
				violations++
			}
		}(i)
	}
	wg.Wait()

	assert.Zero(t, violations, "concurrent plugin globals were overwritten")
}

func TestLoadedFuncCacheConcurrency(t *testing.T) {
	const libPath = "/test/concurrent-handler.so"

	t.Cleanup(func() {
		loadedFuncMu.Lock()
		delete(loadedFunc, libPath)
		loadedFuncMu.Unlock()
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loadedFuncMu.Lock()
			loadedPlugin := loadedFunc[libPath]
			if !loadedPlugin.Loaded {
				loadedFuncMu.Unlock()
				time.Sleep(time.Microsecond)
				loadedFuncMu.Lock()
				if existing, ok := loadedFunc[libPath]; ok && existing.Loaded {
					_ = existing.Plugin
				} else {
					loadedFunc[libPath] = LoadedPlugin{Loaded: true}
				}
			}
			loadedFuncMu.Unlock()
		}()
	}
	wg.Wait()

	loadedFuncMu.Lock()
	entry, ok := loadedFunc[libPath]
	loadedFuncMu.Unlock()
	require.True(t, ok)
	assert.True(t, entry.Loaded)
}

func TestLoadedMiddlewareCacheConcurrency(t *testing.T) {
	t.Parallel()

	const libPath = "/test/concurrent-middleware.so"

	t.Cleanup(func() {
		loadedMiddlewareMu.Lock()
		delete(loadedMiddlewareFunc, libPath)
		loadedMiddlewareMu.Unlock()
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loadedMiddlewareMu.Lock()
			loadedPlugin := loadedMiddlewareFunc[libPath]
			if !loadedPlugin.Loaded {
				loadedMiddlewareMu.Unlock()
				time.Sleep(time.Microsecond)
				loadedMiddlewareMu.Lock()
				if existing, ok := loadedMiddlewareFunc[libPath]; ok && existing.Loaded {
					_ = existing.Plugin
				} else {
					loadedMiddlewareFunc[libPath] = LoadedPlugin{Loaded: true}
				}
			}
			loadedMiddlewareMu.Unlock()
		}()
	}
	wg.Wait()

	loadedMiddlewareMu.Lock()
	entry, ok := loadedMiddlewareFunc[libPath]
	loadedMiddlewareMu.Unlock()
	require.True(t, ok)
	assert.True(t, entry.Loaded)
}
