package router

import "github.com/gorilla/mux"

var router *mux.Router

func initRouter() {
	router = mux.NewRouter().StrictSlash(true)
}

// Get Router for pREST
func Get() *mux.Router {
	if router == nil {
		initRouter()
	}
	return router
}
