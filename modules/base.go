package modules

import (
	"github.com/gorilla/mux"
	"github.com/nuveo/prest/modules/ping"
)

type Module interface {
	Name() string
	Register(r *mux.Router)
}

func Register(router *mux.Router) {
	p := ping.PingModule{}
	p.Register(router)
}
