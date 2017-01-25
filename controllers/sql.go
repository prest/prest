package controllers

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/adapters/postgres"
)

// ExecuteFromScripts is a controller to peform SQL in scripts created by users
func ExecuteFromScripts(rw http.ResponseWriter, rq *http.Request) {
	vars := mux.Vars(rq)
	queriesPath := vars["queriesLocation"]
	script := vars["script"]

	sqlPath, err := postgres.GetScript(rq.Method, queriesPath, script)
	if err != nil {
		log.Printf("could not get script %s/%s, %+v", queriesPath, script, err)
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	sql, values, err := postgres.ParseScript(sqlPath, rq.URL.Query())
	if err != nil {
		log.Printf("could not parse script %s/%s, %+v", queriesPath, script, err)
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := postgres.ExecuteScripts(rq.Method, sql, values)
	if err != nil {
		log.Printf("could not execute sql %+v, %s", err, sql)
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	rw.Write(result)
}
