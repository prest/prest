// nolint
// all plugins must have their package name as `main`
// each plugin is isolated at compile time
package main

import (
	"net/http"

	"github.com/urfave/negroni/v3"
)

// BUILD:
// go build -o ./lib/hello_midlleware.so -buildmode=plugin ./lib/src/middlewares/hello.go
func HelloMiddlewareLoad() negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		next(rw, rq)
	})
}
