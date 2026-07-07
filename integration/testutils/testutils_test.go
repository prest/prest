package testutils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestDoRequest(t *testing.T) {
	router := mux.NewRouter()
	bodyResponse := `{"key": "value"}`
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//nolint
		w.Write([]byte(bodyResponse))
	}).Methods("GET")
	server := httptest.NewServer(router)
	defer server.Close()

	DoRequest(t, server.URL, nil, "GET", http.StatusOK, "")
	DoRequest(t, server.URL+"/not-found", nil, "GET", http.StatusNotFound, "")
	DoRequest(t, server.URL+"/", nil, "POST", http.StatusMethodNotAllowed, "")
	DoRequest(t, server.URL+"/", nil, "GET", http.StatusOK, "", bodyResponse)
}
