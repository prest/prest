package plugins

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"plugin"
	"runtime"

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
var loadedFunc = map[string]LoadedPlugin{}

// loadedMiddlewareFunc global variable to control plugins loaded, blocking duplicate loading
var loadedMiddlewareFunc = map[string]LoadedPlugin{}

// loadFunc private func to load and exec OS Library
func (plg *Plugins) loadFunc(fileName, funcName string, r *http.Request) (ret PluginFuncReturn, err error) {
	libPath := filepath.Join(plg.cfg.PluginPath, fmt.Sprintf("%s.so", fileName))
	loadedPlugin := loadedFunc[libPath]
	p := loadedPlugin.Plugin
	if !loadedPlugin.Loaded {
		p, err = plugin.Open(libPath)
		if err != nil {
			return
		}
		loadedPlugin = LoadedPlugin{
			Loaded: true,
			Plugin: p,
		}
		loadedFunc[libPath] = loadedPlugin
	}

	vars := mux.Vars(r)
	httpVars, err := p.Lookup("HTTPVars")
	if err != nil {
		return
	}
	*httpVars.(*map[string]string) = vars

	urlQuery, err := p.Lookup("URLQuery")
	if err != nil {
		return
	}
	*urlQuery.(*map[string][]string) = r.URL.Query()

	f, err := p.Lookup(fmt.Sprintf("%s%sHandler", r.Method, funcName))
	if err != nil {
		return
	}
	function, ok := f.(func() string)

	if !ok {
		function := f.(func() (string, int))
		retJson, code := function()
		ret.ReturnJson = retJson
		ret.StatusCode = code

		slog.Info("ret plugin(status %d): %s\n", "code", code, "ret.ReturnJson", ret.ReturnJson)
	} else {
		retJson := function()
		ret.ReturnJson = retJson
		ret.StatusCode = -1

		slog.Info("ret plugin:", "ret.ReturnJson", ret.ReturnJson)
	}

	return
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
	loadedPlugin := loadedMiddlewareFunc[libPath]
	p := loadedPlugin.Plugin
	if !loadedPlugin.Loaded {
		p, err = plugin.Open(libPath)
		if err != nil {
			return
		}
		loadedMiddlewareFunc[libPath] = LoadedPlugin{
			Loaded: true,
			Plugin: p,
		}
	}
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
