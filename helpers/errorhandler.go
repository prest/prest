package helpers

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorHandler format error to log and return json via http
func ErrorHandler(w http.ResponseWriter, err error) {
	log.Println(err)

	m := make(map[string]string)
	m["error"] = err.Error()
	b, _ := json.MarshalIndent(m, "", "\t")

	http.Error(w, string(b), http.StatusBadRequest)
}
