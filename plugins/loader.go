package plugins

import (
	"fmt"
	"net/http"
	"path/filepath"
	"plugin"

	"github.com/gorilla/mux"
	slog "github.com/structy/log"
)

type Loader interface {
	LoadFunc(fileName, funcName string, r *http.Request) (ret PluginFuncReturn, err error)
}

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

type Config struct {
	path string
}

func New(path string) *Config {
	return &Config{
		path: path,
	}
}

// loadedFunc global variable to control plugins loaded, blocking duplicate loading
var loadedFunc = map[string]LoadedPlugin{}

// loadFunc private func to load and exec OS Library
func (c Config) LoadFunc(fileName, funcName string, r *http.Request) (ret PluginFuncReturn, err error) {
	libPath := filepath.Join(c.path, fmt.Sprintf("%s.so", fileName))
	loadedPlugin := loadedFunc[libPath]
	p := loadedPlugin.Plugin
	// plugin will be loaded only on the first call to the endpoint
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

	// HTTPVars populate
	vars := mux.Vars(r)
	httpVars, err := p.Lookup("HTTPVars")
	if err != nil {
		return
	}
	*httpVars.(*map[string]string) = vars

	// URL Query populate
	urlQuery, err := p.Lookup("URLQuery")
	if err != nil {
		return
	}

	// can this panic?
	// why is urlQuery not being used after this?
	*urlQuery.(*map[string][]string) = r.URL.Query()

	// function name: HttpMethod+FunctionName+"Handler" (string sufix)
	// standardizing the name of the method that will be invoked we use
	// the name Handler as a suffix to identify what will be called in the http
	f, err := p.Lookup(fmt.Sprintf("%s%sHandler", r.Method, funcName))
	if err != nil {
		return
	}
	// Exec (call) function name, return string (
	// In case which return status code does not matter)
	function, ok := f.(func() string)

	if !ok {
		// It is probable that plugin function return not only json but also status code.
		function := f.(func() (string, int))
		ret.ReturnJson, ret.StatusCode = function()

		slog.Printf("ret plugin(status %d): %s\n", ret.StatusCode, ret.ReturnJson)
	} else {
		retJson := function()
		ret.ReturnJson = retJson
		ret.StatusCode = -1

		slog.Println("ret plugin:", ret.ReturnJson)
	}

	return
}
