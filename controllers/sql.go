package controllers

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/adapters/postgres"
)

// ExecuteScriptQuery is a function to execute and return result of script query
func ExecuteScriptQuery(rq *http.Request, queriesPath string, script string) ([]byte, error) {
	sqlPath, err := postgres.GetScript(rq.Method, queriesPath, script)
	if err != nil {
		log.Printf("could not get script %s/%s, %+v", queriesPath, script, err)
		return nil, err
	}

	sql, values, err := postgres.ParseScript(sqlPath, rq.URL.Query())
	if err != nil {
		log.Printf("could not parse script %s/%s, %+v", queriesPath, script, err)
		return nil, err
	}

	result, err := postgres.ExecuteScripts(rq.Method, sql, values)
	if err != nil {
		log.Printf("could not execute sql %+v, %s", err, sql)
		return nil, err
	}

	return result, nil
}

// ExecuteFromScripts is a controller to peform SQL in scripts created by users
func ExecuteFromScripts(rw http.ResponseWriter, rq *http.Request) {
	vars := mux.Vars(rq)
	queriesPath := vars["queriesLocation"]
	script := vars["script"]

	result, err := ExecuteScriptQuery(rq, queriesPath, script)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	rw.Write(result)
}
