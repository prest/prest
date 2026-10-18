package plugins

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/internal/ident"
	"github.com/prest/prest/v2/internal/pathsafe"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"
)

var (
	jsonErrFormat = `{"error": "%s"}`
	// ErrInvalidPluginPath is returned when a plugin file or function name is not
	// a safe single segment, or the resulting library path escapes PluginPath.
	ErrInvalidPluginPath = errors.New("invalid plugin name")
)

// LoadedPlugin structure for controlling the loaded plugin
type LoadedPlugin struct {
	Loaded bool
	Plugin pluginLib
}

// pluginLib is the subset of *plugin.Plugin used after Open. Tests inject a
// fake via Plugins.open without building a real .so.
type pluginLib interface {
	Lookup(string) (plugin.Symbol, error)
}

// PluginFuncReturn structure for holding return value and status of plugin function.
type PluginFuncReturn struct {
	ReturnJson string
	StatusCode int
}

// Plugins holds plugin configuration for handler and middleware loading.
type Plugins struct {
	cfg  *config.Prest
	open func(string) (pluginLib, error)
}

// New creates a Plugins instance for the given config.
func New(cfg *config.Prest) *Plugins {
	return &Plugins{
		cfg: cfg,
		open: func(path string) (pluginLib, error) {
			return plugin.Open(path)
		},
	}
}

// loadedFunc global variable to control plugins loaded, blocking duplicate loading
var (
	loadedFuncMu sync.Mutex
	loadedFunc   = map[string]LoadedPlugin{}
)

// loadedMiddlewareFunc global variable to control plugins loaded, blocking duplicate loading
var (
	loadedMiddlewareMu   sync.Mutex
	loadedMiddlewareFunc = map[string]LoadedPlugin{}
)

// pluginInvokeMu serializes handler invocation per .so path. Plugin ABI exposes
// HTTPVars and URLQuery as package-level symbols; concurrent requests would
// overwrite each other's values without this lock.
var pluginInvokeMu sync.Map // map[string]*sync.Mutex

func pluginInvokeMutex(libPath string) *sync.Mutex {
	v, _ := pluginInvokeMu.LoadOrStore(libPath, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// resolvePluginPath maps a plugin name to its .so path under base. fileName
// reaches here from the {file} URL segment (or plugin config), so it must be a
// single safe segment and the joined path must stay inside base, both lexically
// and after symlinks are resolved. Charset and containment failures wrap
// ErrInvalidPluginPath; a missing file is a plain error so callers keep
// treating it as not found.
func resolvePluginPath(base, fileName string) (string, error) {
	if !ident.IsSafeSegment(fileName) {
		return "", fmt.Errorf("%w: file %q", ErrInvalidPluginPath, fileName)
	}
	base = filepath.Clean(base)
	libPath := filepath.Join(base, fileName+".so")
	if !pathsafe.WithinBase(base, libPath) {
		return "", fmt.Errorf("%w: file %q escapes plugin path", ErrInvalidPluginPath, fileName)
	}
	if _, err := os.Stat(libPath); err != nil {
		return "", fmt.Errorf("plugin open %s: %w", libPath, err)
	}
	// The file exists, so links can now be resolved: a symlink inside base that
	// targets a library outside it passes the lexical check above.
	if !pathsafe.ResolvedWithinBase(base, libPath) {
		return "", fmt.Errorf("%w: file %q resolves outside plugin path", ErrInvalidPluginPath, fileName)
	}
	return libPath, nil
}

// isPluginFuncName reports whether funcName is a plain Go identifier, the only
// shape that can name an exported plugin handler symbol.
func isPluginFuncName(funcName string) bool {
	return ident.IsValid(funcName) && !strings.Contains(funcName, ".")
}

// loadFunc private func to load and exec OS Library
func (plg *Plugins) loadFunc(fileName, funcName string, r *http.Request) (ret PluginFuncReturn, err error) {
	if !isPluginFuncName(funcName) {
		return ret, fmt.Errorf("%w: func %q", ErrInvalidPluginPath, funcName)
	}
	libPath, err := resolvePluginPath(plg.cfg.PluginPath, fileName)
	if err != nil {
		return ret, err
	}

	loadedFuncMu.Lock()
	loadedPlugin := loadedFunc[libPath]
	p := loadedPlugin.Plugin
	if !loadedPlugin.Loaded {
		loadedFuncMu.Unlock()
		p, err = plg.open(libPath)
		if err != nil {
			return ret, fmt.Errorf("plugin open %s: %w", libPath, err)
		}
		loadedFuncMu.Lock()
		if existing, ok := loadedFunc[libPath]; ok && existing.Loaded {
			p = existing.Plugin
		} else {
			loadedFunc[libPath] = LoadedPlugin{
				Loaded: true,
				Plugin: p,
			}
		}
	}
	loadedFuncMu.Unlock()

	mu := pluginInvokeMutex(libPath)
	mu.Lock()
	defer mu.Unlock()

	vars := mux.Vars(r)
	httpVars, err := p.Lookup("HTTPVars")
	if err != nil {
		return
	}
	if err = assignPluginHTTPVars(httpVars, vars); err != nil {
		return
	}

	urlQuery, err := p.Lookup("URLQuery")
	if err != nil {
		return
	}
	if err = assignPluginURLQuery(urlQuery, r.URL.Query()); err != nil {
		return
	}

	handlerName := fmt.Sprintf("%s%sHandler", r.Method, funcName)
	f, err := p.Lookup(handlerName)
	if err != nil {
		return
	}
	ret, err = invokePluginHandler(f, handlerName)
	return
}

func assignPluginHTTPVars(sym plugin.Symbol, vars map[string]string) error {
	httpVars, ok := sym.(*map[string]string)
	if !ok {
		return fmt.Errorf("plugin HTTPVars symbol is not *map[string]string")
	}
	*httpVars = vars
	return nil
}

func assignPluginURLQuery(sym plugin.Symbol, query map[string][]string) error {
	urlQuery, ok := sym.(*map[string][]string)
	if !ok {
		return fmt.Errorf("plugin URLQuery symbol is not *map[string][]string")
	}
	*urlQuery = query
	return nil
}

func invokePluginHandler(sym plugin.Symbol, handlerName string) (ret PluginFuncReturn, err error) {
	if function, ok := sym.(func() string); ok {
		ret.ReturnJson = function()
		ret.StatusCode = -1
		slog.Info("ret plugin:", "ret.ReturnJson", ret.ReturnJson)
		return
	}
	if function, ok := sym.(func() (string, int)); ok {
		retJson, code := function()
		ret.ReturnJson = retJson
		ret.StatusCode = code
		slog.Info("ret plugin(status %d): %s\n", "code", code, "ret.ReturnJson", ret.ReturnJson)
		return
	}
	return ret, fmt.Errorf("plugin handler %s is not func() string or func() (string, int)", handlerName)
}

// Handler serves plugin endpoints.
func (plg *Plugins) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		fileName := vars["file"]
		funcName := vars["func"]
		ret, err := plg.loadFunc(fileName, funcName, r)
		if errors.Is(err, ErrInvalidPluginPath) {
			// err quotes the rejected names with %q, so control characters in
			// attacker input cannot forge log lines; the response never echoes them.
			slog.Warn("rejected plugin request", "err", err)
			http.Error(w, fmt.Sprintf(jsonErrFormat, ErrInvalidPluginPath.Error()), http.StatusBadRequest)
			return
		}
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusNotFound)
			return
		}
		plg.cfg.Cache.BuntSet(r.URL.String(), ret.ReturnJson)

		if ret.StatusCode != -1 {
			w.WriteHeader(ret.StatusCode)
		}

		w.Write([]byte(ret.ReturnJson))
	}
}

func (plg *Plugins) loadMiddlewareFunc(fileName, funcName string) (handlerFunc negroni.HandlerFunc, err error) {
	libPath, err := resolvePluginPath(filepath.Join(plg.cfg.PluginPath, "middlewares"), fileName)
	if err != nil {
		return nil, err
	}

	loadedMiddlewareMu.Lock()
	loadedPlugin := loadedMiddlewareFunc[libPath]
	p := loadedPlugin.Plugin
	if !loadedPlugin.Loaded {
		loadedMiddlewareMu.Unlock()
		p, err = plg.open(libPath)
		if err != nil {
			return nil, fmt.Errorf("plugin open %s: %w", libPath, err)
		}
		loadedMiddlewareMu.Lock()
		if existing, ok := loadedMiddlewareFunc[libPath]; ok && existing.Loaded {
			p = existing.Plugin
		} else {
			loadedMiddlewareFunc[libPath] = LoadedPlugin{
				Loaded: true,
				Plugin: p,
			}
		}
	}
	loadedMiddlewareMu.Unlock()
	f, err := p.Lookup(fmt.Sprintf("%sMiddlewareLoad", funcName))
	if err != nil {
		slog.Error("unable to load middleware plugin function", "funcName", funcName, "err", err)
		return
	}
	handlerFunc, err = adaptMiddlewareSymbol(f)
	if err != nil {
		slog.Error("it not a negroni middleware function", "funcName", funcName, "err", err)
		return
	}
	return
}

// adaptMiddlewareSymbol converts a Lookup'd middleware export into a
// negroni.HandlerFunc. The documented contract is a factory
// (`func() negroni.Handler`); the legacy direct HandlerFunc shape is still
// accepted for .so files built against the older assert.
func adaptMiddlewareSymbol(f any) (negroni.HandlerFunc, error) {
	if factory, ok := f.(func() negroni.Handler); ok {
		h := factory()
		if h == nil {
			return nil, fmt.Errorf("negroni middleware factory returned nil")
		}
		return h.ServeHTTP, nil
	}
	if direct, ok := f.(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc)); ok {
		return negroni.HandlerFunc(direct), nil
	}
	return nil, fmt.Errorf("symbol is not a negroni middleware factory or HandlerFunc")
}

// Middleware loads configured plugin middleware.
func (plg *Plugins) Middleware() negroni.Handler {
	if runtime.GOOS != "windows" {
		for _, plugin := range plg.cfg.PluginMiddlewareList {
			fn, err := plg.loadMiddlewareFunc(plugin.File, plugin.Func)
			if err != nil {
				slog.Error("unable to load middleware plugin function", "funcName", plugin.Func, "err", err)
				continue
			}
			if fn == nil {
				continue
			}
			return negroni.HandlerFunc(fn)
		}
	}
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		next(rw, rq)
	})
}
