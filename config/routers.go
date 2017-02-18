package config

import "github.com/gorilla/mux"

var router *mux.Router

func initRouter() {
	router = mux.NewRouter()
}

func GetRouter() *mux.Router {
	if router == nil {
		initRouter()
	}
	return router
}
