package plugins

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"plugin"

	"github.com/prest/prest/v2/config"

	"github.com/gorilla/mux"
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

// loadedFunc global variable to control plugins loaded, blocking duplicate loading
var loadedFunc = map[string]LoadedPlugin{}

// loadFunc private func to load and exec OS Library
func loadFunc(fileName, funcName string, r *http.Request) (ret PluginFuncReturn, err error) {
	libPath := filepath.Join(config.PrestConf.PluginPath, fmt.Sprintf("%s.so", fileName))
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
	*urlQuery.(*map[string][]string) = r.URL.Query()

	// function name: HttpMethod+FunctionName+"Handler" (string sufix)
	// standardizing the name of the method that will be invoked we use
	// the name Handler as a suffix to identify what will be called in the http
	f, err := p.Lookup(fmt.Sprintf("%s%sHandler", r.Method, funcName))
	if err != nil {
		return
	}
	// Exec (call) function name, return string (In case which return status code does not matter)
	function, ok := f.(func() string)

	if !ok {
		// It is probable that plugin function return not only json but also status code.
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

// HandlerPlugin responsible for processing the `.so` function via http protocol
func HandlerPlugin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["file"]
	funcName := vars["func"]
	ret, err := loadFunc(fileName, funcName, r)
	if err != nil {
		slog.Error(err.Error())
		http.Error(w, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusNotFound)
		return
	}
	// Cache arrow if enabled
	config.PrestConf.Cache.BuntSet(r.URL.String(), ret.ReturnJson)

	//nolint
	if ret.StatusCode != -1 {
		w.WriteHeader(ret.StatusCode)
	}

	w.Write([]byte(ret.ReturnJson))
}
