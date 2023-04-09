package plugins

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"plugin"

	"github.com/prest/prest/config"
	"github.com/urfave/negroni/v3"
)

// loadedFunc global variable to control plugins loaded, blocking duplicate loading
var loadedMiddlewareFunc = map[string]LoadedPlugin{}

// loadFunc private func to load and exec OS Library
func loadMiddlewareFunc(fileName, funcName string) (handlerFunc negroni.HandlerFunc, err error) {
	libPath := filepath.Join(config.PrestConf.PluginPath, "middleware", fmt.Sprintf("%s.so", fileName))
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

	/**
	handlerFunc = func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		next(rw, rq)
	}
	return
	*/

	// function name: FunctionName+"MiddlewareLoad" (string sufix)
	// standardizing the name of the method that will be invoked we use
	// the name Handler as a suffix to identify what will be called in the http
	f, err := p.Lookup(fmt.Sprintf("%sMiddlewareLoad", funcName))
	if err != nil {
		return
	}
	// Exec (call) function name, return `negroni.HandlerFunc`
	handlerFunc, ok := f.(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc))
	if !ok {
		// It is probable that plugin function return not only json but also status code.
		// log.Printf("ret plugin(status %d): %s\n", code, ret.ReturnJson)
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
	// list of plugins configured to be loaded
	pluginMiddlewareList := config.PrestConf.PluginMiddlewareList
	for _, plugin := range pluginMiddlewareList {
		fn, err := loadMiddlewareFunc(plugin.File, plugin.Func)
		if err != nil {
			log.Println(err)
			return nil
		}
		return negroni.HandlerFunc(fn)
	}
	return nil
}
