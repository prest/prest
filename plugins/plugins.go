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

// loadFunc private func to load and exec OS Library
func loadFunc(fileName, funcName string, r *http.Request) (ret string, err error) {
	p, err := plugin.Open(filepath.Join(config.PrestConf.PluginPath, fmt.Sprintf("%s.so", fileName)))
	if err != nil {
		return
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
	fmt.Println("ret plugin:", ret)
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
