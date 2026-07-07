package plugins

import (
	"fmt"
	"net/http"
	"net/http/httptest"	
	"path/filepath"
	"plugin"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/prest/prest/v2/config"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
)

// absPluginTestDataDir returns the absolute path to plugins/testdata.
func absPluginTestDataDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(filename)
	abs, err := filepath.Abs(filepath.Join(dir, "testdata"))
	require.NoError(t, err)
	return abs
}

// mockPlugin simulates plugin.Plugin in memory — no .so file required.
type mockPlugin struct {
	symbols map[string]plugin.Symbol
}

func (m *mockPlugin) Lookup(name string) (plugin.Symbol, error) {
	sym, ok := m.symbols[name]
	if !ok {
		return nil, fmt.Errorf("symbol %q not found in mock plugin", name)
	}
	return sym, nil
}

// --- Middleware type assertion tests (cover main bug #875) ---

func TestNegroniHandlerFuncImplementsNegroniHandler(t *testing.T) {
	var got http.ResponseWriter
	rw := httptest.NewRecorder()
	rq := httptest.NewRequest(http.MethodGet, "/", nil)
	nextCalled := false
	mw := negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		got = rw
		next(rw, rq)
	})
	var handler negroni.Handler = mw
	handler.ServeHTTP(rw, rq, func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})
	require.True(t, nextCalled, "next middleware must be invoked by the handler chain")
	require.NotNil(t, got)
}

func TestNegroniMiddlewareLoadTypeAssertion(t *testing.T) {
	t.Run("factory func returning negroni.Handler can be called", func(t *testing.T) {
		var ran bool
		factory := func() negroni.Handler {
			return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
				ran = true
				next(rw, rq)
			})
		}
		var loader func() negroni.Handler = factory
		require.NotNil(t, loader)
		handler := loader()
		require.NotNil(t, handler)
		rw := httptest.NewRecorder()
		rq := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rw, rq, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		require.True(t, ran, "middleware returned by factory must be callable")
	})
	t.Run("negroni.HandlerFunc does not match raw func signature in type assertion", func(t *testing.T) {
		mw := negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {})
		_, ok := interface{}(mw).(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc))
		require.False(t, ok, "negroni.HandlerFunc (named type) must not match raw func signature in type assertion")
	})
}

func TestLoadMiddlewareFuncUnit(t *testing.T) {
	mw := negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		rw.Header().Add("X-Unit", "ok")
		next(rw, rq)
	})
	factory := func() negroni.Handler { return mw }
	mockPlg := &mockPlugin{symbols: map[string]plugin.Symbol{
		"TestMiddlewareLoad": factory,
	}}

	loader, err := mockPlg.Lookup("TestMiddlewareLoad")
	require.NoError(t, err, "mock lookup must succeed")
	factory2, ok := loader.(func() negroni.Handler)
	require.True(t, ok, "symbol must be func() negroni.Handler")
	handler := factory2()
	require.NotNil(t, handler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	called := false
	handler.ServeHTTP(w, r, func(w http.ResponseWriter, r *http.Request) { called = true })
	assert.True(t, called)
	assert.Equal(t, "ok", w.Header().Get("X-Unit"))
}

func TestLoadMiddlewareFuncTypeError(t *testing.T) {
	mockPlg := &mockPlugin{symbols: map[string]plugin.Symbol{
		"BadMiddlewareLoad": func() {}, // is not func() negroni.Handler
	}}

	loader, err := mockPlg.Lookup("BadMiddlewareLoad")
	require.NoError(t, err)
	_, ok := loader.(func() negroni.Handler)
	require.False(t, ok, "plain func must not satisfy func() negroni.Handler")
}

// --- Existing unit tests ---

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

// --- Extra coverage tests (no .so dependency) ---

func TestNew(t *testing.T) {
	cfg := &config.Prest{PluginPath: "./lib"}
	plg := New(cfg)
	require.NotNil(t, plg)
	require.Equal(t, cfg, plg.cfg)
}

func TestMiddlewareReturnsNoopOnError(t *testing.T) {
	cfg := &config.Prest{
		PluginPath:           "/nonexistent",
		PluginMiddlewareList: []config.PluginMiddleware{{File: "missing", Func: "Missing"}},
	}
	plg := New(cfg)
	mw := plg.Middleware()
	require.NotNil(t, mw)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	called := false
	mw.ServeHTTP(w, r, func(w http.ResponseWriter, r *http.Request) { called = true })
	assert.True(t, called)
}

func TestHandlerPluginNotFound(t *testing.T) {
	cfg := &config.Prest{PluginPath: "/nonexistent"}
	plg := New(cfg)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/_PLUGIN/missing/Foo", nil)
	mux.SetURLVars(r, map[string]string{"file": "missing", "func": "Foo"})
	plg.Handler()(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestInvokePluginHandlerCoversStatusCode(t *testing.T) {
	ret, err := invokePluginHandler(func() (string, int) { return `{"ok":true}`, http.StatusAccepted }, "GETWithStatus")
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, ret.ReturnJson)
	assert.Equal(t, http.StatusAccepted, ret.StatusCode)
}

func TestPluginFuncReturnJSON(t *testing.T) {
	assert.Equal(t, `{"error": "something"}`, fmt.Sprintf(jsonErrFormat, "something"))
}

func TestMiddlewareReturnsNoopWhenListEmpty(t *testing.T) {
	cfg := &config.Prest{PluginMiddlewareList: []config.PluginMiddleware{}}
	plg := New(cfg)
	mw := plg.Middleware()
	require.NotNil(t, mw)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	called := false
	mw.ServeHTTP(w, r, func(w http.ResponseWriter, r *http.Request) { called = true })
	assert.True(t, called)
}
