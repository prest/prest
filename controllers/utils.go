package controllers

import (
	"encoding/json"
	"log"
	"net/http"
)

func errorHandler(w http.ResponseWriter, err error) {
	log.Println(err)

	m := make(map[string]string)
	m["error"] = err.Error()
	b, _ := json.MarshalIndent(m, "", "\t")

	http.Error(w, string(b), http.StatusBadRequest)
}
