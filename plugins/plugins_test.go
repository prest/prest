package plugins

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
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

func TestLoadMiddlewareFuncWithFakePlugin(t *testing.T) {
	dir := t.TempDir()
	mwDir := filepath.Join(dir, "middlewares")
	require.NoError(t, os.MkdirAll(mwDir, 0o755))
	soPath := filepath.Join(mwDir, "hello.so")
	require.NoError(t, os.WriteFile(soPath, []byte("placeholder"), 0o644))

	t.Cleanup(func() {
		loadedMiddlewareMu.Lock()
		delete(loadedMiddlewareFunc, soPath)
		loadedMiddlewareMu.Unlock()
	})

	plg := New(&config.Prest{PluginPath: dir})
	plg.open = func(path string) (pluginLib, error) {
		assert.Equal(t, soPath, path)
		return fakePluginLib{symbols: map[string]any{
			"HelloMiddlewareLoad": func() negroni.Handler {
				return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
					rw.Header().Set("X-Hello-Middleware", "Hello Middleware")
					next(rw, rq)
				})
			},
		}}, nil
	}
	fn, err := plg.loadMiddlewareFunc("hello", "Hello")
	require.NoError(t, err)
	require.NotNil(t, fn)

	rec := httptest.NewRecorder()
	fn(rec, httptest.NewRequest(http.MethodGet, "/", nil), func(http.ResponseWriter, *http.Request) {})
	assert.Equal(t, "Hello Middleware", rec.Header().Get("X-Hello-Middleware"))

	// Cached reload path.
	fn2, err := plg.loadMiddlewareFunc("hello", "Hello")
	require.NoError(t, err)
	require.NotNil(t, fn2)
}

func TestMiddlewareLoadsConfiguredPlugin(t *testing.T) {
	dir := t.TempDir()
	mwDir := filepath.Join(dir, "middlewares")
	require.NoError(t, os.MkdirAll(mwDir, 0o755))
	soPath := filepath.Join(mwDir, "hello.so")
	require.NoError(t, os.WriteFile(soPath, []byte("placeholder"), 0o644))

	t.Cleanup(func() {
		loadedMiddlewareMu.Lock()
		delete(loadedMiddlewareFunc, soPath)
		loadedMiddlewareMu.Unlock()
	})

	plg := New(&config.Prest{
		PluginPath: dir,
		PluginMiddlewareList: []config.PluginMiddleware{
			{File: "hello", Func: "Hello"},
		},
	})
	plg.open = func(path string) (pluginLib, error) {
		return fakePluginLib{symbols: map[string]any{
			"HelloMiddlewareLoad": func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
				rw.Header().Set("X-Hello-Middleware", "Hello Middleware")
				next(rw, rq)
			},
		}}, nil
	}
	h := plg.Middleware()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil), func(http.ResponseWriter, *http.Request) {})
	assert.Equal(t, "Hello Middleware", rec.Header().Get("X-Hello-Middleware"))
}

func TestLoadFuncWithFakePlugin(t *testing.T) {
	dir := t.TempDir()
	soPath := filepath.Join(dir, "hello.so")
	require.NoError(t, os.WriteFile(soPath, []byte("placeholder"), 0o644))

	t.Cleanup(func() {
		loadedFuncMu.Lock()
		delete(loadedFunc, soPath)
		loadedFuncMu.Unlock()
	})

	httpVars := map[string]string{}
	urlQuery := map[string][]string{}
	plg := New(&config.Prest{PluginPath: dir})
	plg.open = func(path string) (pluginLib, error) {
		return fakePluginLib{symbols: map[string]any{
			"HTTPVars":        &httpVars,
			"URLQuery":        &urlQuery,
			"GETHelloHandler": func() string { return `{"ok":true}` },
			"GETHelloWithStatusHandler": func() (string, int) {
				return `{"ok":true}`, http.StatusAccepted
			},
		}}, nil
	}
	r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/_PLUGIN/hello/Hello", nil), map[string]string{
		"file": "hello", "func": "Hello",
	})
	ret, err := plg.loadFunc("hello", "Hello", r)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, ret.ReturnJson)
	assert.Equal(t, -1, ret.StatusCode)
	assert.Equal(t, "hello", httpVars["file"])
}

type fakePluginLib struct {
	symbols map[string]any
}

func (f fakePluginLib) Lookup(name string) (plugin.Symbol, error) {
	s, ok := f.symbols[name]
	if !ok {
		return nil, fmt.Errorf("symbol %s not found", name)
	}
	return s, nil
}

func TestNewAndMiddlewareNoop(t *testing.T) {
	t.Parallel()

	plg := New(&config.Prest{})
	require.NotNil(t, plg)

	h := plg.Middleware()
	require.NotNil(t, h)

	rec := httptest.NewRecorder()
	called := false
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil), func(http.ResponseWriter, *http.Request) {
		called = true
	})
	assert.True(t, called, "empty pluginmiddlewarelist must fall through to next")
}

func TestMiddlewareMissingPluginFallsBackToNoop(t *testing.T) {
	t.Parallel()

	plg := New(&config.Prest{
		PluginPath: t.TempDir(),
		PluginMiddlewareList: []config.PluginMiddleware{
			{File: "missing", Func: "Hello"},
		},
	})
	h := plg.Middleware()
	rec := httptest.NewRecorder()
	called := false
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil), func(http.ResponseWriter, *http.Request) {
		called = true
	})
	assert.True(t, called)
}

func TestLoadMiddlewareFuncMissingSO(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	plg := New(&config.Prest{PluginPath: dir})
	_, err := plg.loadMiddlewareFunc("missing", "Hello")
	require.Error(t, err)
	require.ErrorContains(t, err, "plugin open")
	require.ErrorContains(t, err, filepath.Join(dir, "middlewares", "missing.so"))
}

func TestHandlerMissingPlugin(t *testing.T) {
	t.Parallel()

	plg := New(&config.Prest{PluginPath: t.TempDir()})
	r := mux.NewRouter()
	r.HandleFunc("/_PLUGIN/{file}/{func}", plg.Handler())
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/_PLUGIN/missing/Hello")
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAdaptMiddlewareSymbol(t *testing.T) {
	t.Parallel()

	t.Run("factory returning negroni.Handler", func(t *testing.T) {
		t.Parallel()
		factory := func() negroni.Handler {
			return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
				rw.Header().Set("X-From-Factory", "1")
				next(rw, rq)
			})
		}
		fn, err := adaptMiddlewareSymbol(factory)
		require.NoError(t, err)
		require.NotNil(t, fn)

		rec := httptest.NewRecorder()
		called := false
		fn(rec, httptest.NewRequest(http.MethodGet, "/", nil), func(http.ResponseWriter, *http.Request) {
			called = true
		})
		assert.True(t, called)
		assert.Equal(t, "1", rec.Header().Get("X-From-Factory"))
	})

	t.Run("direct HandlerFunc shape", func(t *testing.T) {
		t.Parallel()
		direct := func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
			rw.Header().Set("X-From-Direct", "1")
			next(rw, rq)
		}
		fn, err := adaptMiddlewareSymbol(direct)
		require.NoError(t, err)
		require.NotNil(t, fn)

		rec := httptest.NewRecorder()
		called := false
		fn(rec, httptest.NewRequest(http.MethodGet, "/", nil), func(http.ResponseWriter, *http.Request) {
			called = true
		})
		assert.True(t, called)
		assert.Equal(t, "1", rec.Header().Get("X-From-Direct"))
	})

	t.Run("unsupported symbol type", func(t *testing.T) {
		t.Parallel()
		_, err := adaptMiddlewareSymbol(func() string { return "nope" })
		require.Error(t, err)
		assert.Contains(t, err.Error(), "negroni middleware")
	})

	t.Run("nil factory result", func(t *testing.T) {
		t.Parallel()
		_, err := adaptMiddlewareSymbol(func() negroni.Handler { return nil })
		require.Error(t, err)
	})
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

// recordingOpen returns a plg.open fake that records every path it is asked
// to load, so tests can prove a rejected name never reaches plugin.Open.
func recordingOpen(lib pluginLib) (func(string) (pluginLib, error), func() []string) {
	var mu sync.Mutex
	var calls []string
	open := func(path string) (pluginLib, error) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, path)
		if lib == nil {
			return nil, errors.New("fake open: no plugin")
		}
		return lib, nil
	}
	return open, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), calls...)
	}
}

// GHSA-fmr6-9j48-xrqp: {file} and {func} come from the URL and must never be
// able to steer plugin.Open outside PluginPath.
func TestLoadFuncRejectsUnsafeNames(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		description string
		file        string
		fn          string
	}{
		{"file: reporter traversal to /tmp/sample is rejected", "../../../../tmp/sample", "someFunction"},
		{"file: bare parent dir is rejected", "..", "Hello"},
		{"file: nested path with slash is rejected", "a/b", "Hello"},
		{"file: dotted name is rejected", "a.b", "Hello"},
		{"file: empty name is rejected", "", "Hello"},
		{"file: 64-char name exceeds segment limit", strings.Repeat("a", 64), "Hello"},
		{"func: traversal in func name is rejected", "hello", "../x"},
		{"func: dotted func name is rejected", "hello", "a.b"},
		{"func: empty func name is rejected", "hello", ""},
		{"func: func name starting with digit is rejected", "hello", "1abc"},
	}
	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			plg := New(&config.Prest{PluginPath: dir})
			open, calls := recordingOpen(nil)
			plg.open = open
			r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/_PLUGIN/x/y", nil), map[string]string{
				"file": tc.file, "func": tc.fn,
			})
			_, err := plg.loadFunc(tc.file, tc.fn, r)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidPluginPath), "want ErrInvalidPluginPath, got %v", err)
			assert.Empty(t, calls(), "plugin.Open must not be reached for a rejected name")
		})
	}
}

// A symlink placed inside PluginPath that targets a .so elsewhere passes the
// lexical check; the resolved check must still reject it before open.
func TestLoadFuncRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "evil.so")
	require.NoError(t, os.WriteFile(outside, []byte("placeholder"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "hello.so")))

	plg := New(&config.Prest{PluginPath: dir})
	open, calls := recordingOpen(nil)
	plg.open = open
	r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/_PLUGIN/hello/Hello", nil), map[string]string{
		"file": "hello", "func": "Hello",
	})
	_, err := plg.loadFunc("hello", "Hello", r)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidPluginPath), "want ErrInvalidPluginPath, got %v", err)
	assert.Empty(t, calls())
}

// Reporter PoC: Handler() with file=../../../../tmp/sample must answer 400
// without echoing the name and without calling plugin.Open.
func TestHandlerRejectsTraversalPoC(t *testing.T) {
	plg := New(&config.Prest{PluginPath: t.TempDir()})
	open, calls := recordingOpen(fakePluginLib{symbols: map[string]any{
		"GETsomeFunctionHandler": func() string { return "pwned" },
	}})
	plg.open = open

	r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/_PLUGIN/x/someFunction", nil), map[string]string{
		"file": "../../../../tmp/sample", "func": "someFunction",
	})
	rec := httptest.NewRecorder()
	plg.Handler().ServeHTTP(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	assert.Contains(t, string(body), "invalid plugin name")
	assert.NotContains(t, string(body), "tmp/sample", "attacker input must not be echoed")
	assert.NotContains(t, string(body), "pwned")
	assert.Empty(t, calls())
}

func TestLoadMiddlewareFuncRejectsTraversal(t *testing.T) {
	plg := New(&config.Prest{PluginPath: t.TempDir()})
	open, calls := recordingOpen(nil)
	plg.open = open

	_, err := plg.loadMiddlewareFunc("../../../../tmp/sample", "Hello")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidPluginPath), "want ErrInvalidPluginPath, got %v", err)
	assert.Empty(t, calls())

	// Middleware() falls back to the no-op when the configured name is unsafe.
	plg.cfg.PluginMiddlewareList = []config.PluginMiddleware{{File: "../../../../tmp/sample", Func: "Hello"}}
	called := false
	plg.Middleware().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil),
		func(http.ResponseWriter, *http.Request) { called = true })
	assert.True(t, called)
	assert.Empty(t, calls())
}

// A missing plugin under a valid name is a plain not-found, not an invalid
// name, so Handler keeps answering 404; open is not reached for it either.
func TestLoadFuncMissingPluginIsNotInvalid(t *testing.T) {
	dir := t.TempDir()
	plg := New(&config.Prest{PluginPath: dir})
	open, calls := recordingOpen(nil)
	plg.open = open
	r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/_PLUGIN/missing/Hello", nil), map[string]string{
		"file": "missing", "func": "Hello",
	})
	_, err := plg.loadFunc("missing", "Hello", r)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidPluginPath))
	assert.Empty(t, calls())
}

// Regression: a valid name whose .so exists inside PluginPath still reaches
// open with the expected path.
func TestLoadFuncValidNameReachesOpen(t *testing.T) {
	dir := t.TempDir()
	soPath := filepath.Join(dir, "valid-plugin_1.so")
	require.NoError(t, os.WriteFile(soPath, nil, 0o644))
	t.Cleanup(func() {
		loadedFuncMu.Lock()
		delete(loadedFunc, soPath)
		loadedFuncMu.Unlock()
	})

	httpVars := map[string]string{}
	urlQuery := map[string][]string{}
	plg := New(&config.Prest{PluginPath: dir})
	open, calls := recordingOpen(fakePluginLib{symbols: map[string]any{
		"HTTPVars":        &httpVars,
		"URLQuery":        &urlQuery,
		"GETHelloHandler": func() string { return `{"ok":true}` },
	}})
	plg.open = open
	r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/_PLUGIN/valid-plugin_1/Hello", nil), map[string]string{
		"file": "valid-plugin_1", "func": "Hello",
	})
	ret, err := plg.loadFunc("valid-plugin_1", "Hello", r)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, ret.ReturnJson)
	assert.Equal(t, []string{soPath}, calls())
}
