package controllers

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	slog "github.com/structy/log"
)

var (
	// ErrPluginNotFound is returned when the plugin is not found
	ErrPluginNotFound = errors.New("plugin not found")
)

// Plugin responsible for processing the `.so` function via http protocol
func (c *Config) Plugin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["file"]
	funcName := vars["func"]

	ret, err := c.pluginLoader.LoadFunc(fileName, funcName, r)
	if err != nil {
		slog.Errorln(err.Error())
		JSONError(w, ErrPluginNotFound.Error(), http.StatusNotFound)
		return
	}

	// Cache arrow if enabled
	c.cache.Set(r.URL.String(), ret.ReturnJson)

	code := http.StatusOK
	//nolint
	if ret.StatusCode != -1 {
		code = ret.StatusCode
	}
	JSONWrite(w, ret.ReturnJson, code)
}
