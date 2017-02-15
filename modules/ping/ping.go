package ping

import (
	"net/http"

	"github.com/gorilla/mux"
)

type PingModule struct {
}

func (p *PingModule) Name() string {
	return "Ping example module"
}

func (p *PingModule) Register(r *mux.Router) {
	r.HandleFunc("/ping", Pong).Methods("GET")
}

func Pong(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Pong!"))
	return
}
