package middlewares

import (
	"encoding/json"
	"net/http"

	"github.com/urfave/negroni/v3"
)

// SetMiddlewareStackForTest replaces the middleware stack for integration tests.
func SetMiddlewareStackForTest(stack []negroni.Handler) {
	MiddlewareStack = stack
}

// CustomMiddlewareForTest is a JSON response middleware used in integration tests.
func CustomMiddlewareForTest(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	m := map[string]string{"msg": "Calling custom middleware"}
	b, _ := json.Marshal(m)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
	next(w, r)
}
