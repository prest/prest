package plugins

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"plugin"

	"github.com/gorilla/mux"
	"github.com/prest/prest/cache"
	"github.com/prest/prest/config"
)

// LoadedPlugin structure for controlling the loaded plugin
type LoadedPlugin struct {
	Loaded bool
	Plugin *plugin.Plugin
}

// loadedFunc global variable to control plugins loaded, blocking duplicate loading
var loadedFunc = map[string]LoadedPlugin{}

// loadFunc private func to load and exec OS Library
func loadFunc(fileName, funcName string, r *http.Request) (ret string, err error) {
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
	// Exec (call) function name, return string
	ret = f.(func() string)()
	log.Println("ret plugin:", ret)
	return
}

// HandlerPlugin responsible for processing the `.so` function via http protocol
func HandlerPlugin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["file"]
	funcName := vars["func"]
	ret, err := loadFunc(fileName, funcName, r)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	// Cache arrow if enabled
	cache.BuntSet(r.URL.String(), ret)
	w.Write([]byte(ret))
}
