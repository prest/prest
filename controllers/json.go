package controllers

import (
	"encoding/json"
	"net/http"
)

type jsonErr struct {
	Error string `json:"error"`
}

func JSONError(w http.ResponseWriter, msg interface{}, code int) {
	switch msg.(type) {
	case string:
		msg = jsonErr{Error: msg.(string)}
	case error:
		msg = jsonErr{Error: msg.(error).Error()}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(msg)
}

func JSONWrite(w http.ResponseWriter, msg interface{}, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(msg)
}
