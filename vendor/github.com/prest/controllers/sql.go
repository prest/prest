package controllers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prest/config"
)

// ExecuteScriptQuery is a function to execute and return result of script query
func ExecuteScriptQuery(rq *http.Request, queriesPath string, script string) ([]byte, error) {
	sqlPath, err := config.PrestConf.Adapter.GetScript(rq.Method, queriesPath, script)
	if err != nil {
		err = fmt.Errorf("could not get script %s/%s, %+v", queriesPath, script, err)
		return nil, err
	}

	sql, values, err := config.PrestConf.Adapter.ParseScript(sqlPath, rq.URL.Query())
	if err != nil {
		err = fmt.Errorf("could not parse script %s/%s, %+v", queriesPath, script, err)
		return nil, err
	}

	sc := config.PrestConf.Adapter.ExecuteScripts(rq.Method, sql, values)
	if sc.Err() != nil {
		err = fmt.Errorf("could not execute sql %+v, %s", sc.Err(), sql)
		return nil, err
	}

	return sc.Bytes(), nil
}

// ExecuteFromScripts is a controller to peform SQL in scripts created by users
func ExecuteFromScripts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queriesPath := vars["queriesLocation"]
	script := vars["script"]

	result, err := ExecuteScriptQuery(r, queriesPath, script)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(result)
}
