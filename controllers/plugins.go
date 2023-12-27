package controllers

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// Plugin responsible for processing the `.so` function via http protocol
func (c *Config) Plugin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileName := vars["file"]
	funcName := vars["func"]

	ret, err := c.plugins.LoadFunc(fileName, funcName, r)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Cache arrow if enabled
	c.server.Cache.BuntSet(r.URL.String(), ret.ReturnJson)

	//nolint
	if ret.StatusCode != -1 {
		w.WriteHeader(ret.StatusCode)
	}

	w.Write([]byte(ret.ReturnJson))
}
