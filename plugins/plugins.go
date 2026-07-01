package plugins

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"plugin"
	"runtime"
	"sync"

	"github.com/prest/prest/v2/config"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"
)

var (
	jsonErrFormat = `{"error": "%s"}`
)

// LoadedPlugin structure for controlling the loaded plugin
type LoadedPlugin struct {
	Loaded bool
	Plugin *plugin.Plugin
}

// PluginFuncReturn structure for holding return value and status of plugin function.
type PluginFuncReturn struct {
	ReturnJson string
	StatusCode int
}

// Plugins holds plugin configuration for handler and middleware loading.
type Plugins struct {
	cfg *config.Prest
}

// New creates a Plugins instance for the given config.
func New(cfg *config.Prest) *Plugins {
	return &Plugins{cfg: cfg}
}

// loadedFunc global variable to control plugins loaded, blocking duplicate loading
var (
	loadedFuncMu sync.Mutex
	loadedFunc   = map[string]LoadedPlugin{}
)

// loadedMiddlewareFunc global variable to control plugins loaded, blocking duplicate loading
var (
	loadedMiddlewareMu sync.Mutex
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

// loadFunc private func to load and exec OS Library
func (plg *Plugins) loadFunc(fileName, funcName string, r *http.Request) (ret PluginFuncReturn, err error) {
	libPath := filepath.Join(plg.cfg.PluginPath, fmt.Sprintf("%s.so", fileName))

	loadedFuncMu.Lock()
	loadedPlugin := loadedFunc[libPath]
	p := loadedPlugin.Plugin
	if !loadedPlugin.Loaded {
		loadedFuncMu.Unlock()
		p, err = plugin.Open(libPath)
		if err != nil {
			return
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
	libPath := filepath.Join(plg.cfg.PluginPath, "middlewares", fmt.Sprintf("%s.so", fileName))

	loadedMiddlewareMu.Lock()
	loadedPlugin := loadedMiddlewareFunc[libPath]
	p := loadedPlugin.Plugin
	if !loadedPlugin.Loaded {
		loadedMiddlewareMu.Unlock()
		p, err = plugin.Open(libPath)
		if err != nil {
			return
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
		slog.Error("unable to load middleware plugin function: %s", "funcName", funcName)
		return
	}
	handlerFunc, ok := f.(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc))
	if !ok {
		slog.Error("it not a negroni middleware function: %s", "funcName", funcName)
		return
	}
	return
}

// Middleware loads configured plugin middleware.
func (plg *Plugins) Middleware() negroni.Handler {
	if runtime.GOOS != "windows" {
		for _, plugin := range plg.cfg.PluginMiddlewareList {
			fn, err := plg.loadMiddlewareFunc(plugin.File, plugin.Func)
			if err != nil {
				slog.Error("unable to load middleware plugin function: %s", "funcName", plugin.Func)
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
