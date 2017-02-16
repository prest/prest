package config

import "github.com/gorilla/mux"

var Router *mux.Router

func initRouter() {
	Router = mux.NewRouter()
}

func GetRouter() *mux.Router {
	if Router == nil {
		initRouter()
	}
	return Router
}
