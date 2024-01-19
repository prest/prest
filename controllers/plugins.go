package controllers

import (
	"net/http"

	"github.com/gorilla/mux"
	slog "github.com/structy/log"
)

// Plugin responsible for processing the `.so` function via http protocol
func (c *Config) Plugin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["file"]
	funcName := vars["func"]

	ret, err := c.pluginLoader.LoadFunc(fileName, funcName, r)
	if err != nil {
		slog.Errorln(err.Error())
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Cache arrow if enabled
	c.cache.BuntSet(r.URL.String(), ret.ReturnJson)

	//nolint
	if ret.StatusCode != -1 {
		w.WriteHeader(ret.StatusCode)
	}

	w.Write([]byte(ret.ReturnJson))
}
