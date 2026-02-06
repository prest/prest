package plugins

import (
	"fmt"
	"net/http"
	"path/filepath"
	"plugin"
	"runtime"

	"github.com/prest/prest/v2/config"

	"log/slog"

	"github.com/urfave/negroni/v3"
)

// loadedFunc global variable to control plugins loaded, blocking duplicate loading
var loadedMiddlewareFunc = map[string]LoadedPlugin{}

// loadFunc private func to load and exec OS Library
func loadMiddlewareFunc(fileName, funcName string) (handlerFunc negroni.HandlerFunc, err error) {
	// path to plugin file ex: `./libs/middlewares/hello.so`
	libPath := filepath.Join(config.PrestConf.PluginPath, "middlewares", fmt.Sprintf("%s.so", fileName))
	loadedPlugin := loadedMiddlewareFunc[libPath]
	p := loadedPlugin.Plugin
	// plugin will be loaded only on the first call to the endpoint
	if !loadedPlugin.Loaded {
		p, err = plugin.Open(libPath)
		if err != nil {
			return
		}
		loadedFunc[libPath] = LoadedPlugin{
			Loaded: true,
			Plugin: p,
		}
	}
	// function name: FunctionName+"MiddlewareLoad" (string sufix)
	// standardizing the name of the method that will be invoked we use
	// the name Handler as a suffix to identify what will be called in the http
	f, err := p.Lookup(fmt.Sprintf("%sMiddlewareLoad", funcName))
	if err != nil {
		slog.Error("unable to load middleware plugin function: %s", "funcName", funcName)
		return
	}
	// Exec (call) function name, return `negroni.HandlerFunc`
	handlerFunc, ok := f.(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc))
	if !ok {
		slog.Error("it not a negroni middleware function: %s", "funcName", funcName)
		return
	}
	return
}

// MiddlewarePlugin responsible for processing the `.so` middleware pattern
/**
example .toml config:
[[pluginmiddlewarelist]]
file = "hello_midlleware.so"
func = "Hello"
*/
func MiddlewarePlugin() negroni.Handler {
	if runtime.GOOS != "windows" {
		// list of plugins configured to be loaded
		pluginMiddlewareList := config.PrestConf.PluginMiddlewareList
		for _, plugin := range pluginMiddlewareList {
			fn, err := loadMiddlewareFunc(plugin.File, plugin.Func)
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
	// negroni not support nil, return empty middleware to continue request
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		next(rw, rq)
	})
}
