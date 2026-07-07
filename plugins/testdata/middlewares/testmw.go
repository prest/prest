package main
import (
	"net/http"
	"github.com/urfave/negroni/v3"
)

func TestMiddlewareLoad() negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		rw.Header().Add("X-Test", "1")
		next(rw, rq)
	})
}
