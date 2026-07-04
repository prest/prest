package middlewares

import (
	"encoding/json"
	"net/http"

	"github.com/prest/prest/v2/config"

	"github.com/urfave/negroni/v3"
)

// NewForTest builds a negroni stack with optional extra handlers for integration tests.
func NewForTest(cfg *config.Prest, extra ...negroni.Handler) *negroni.Negroni {
	stack := BaseStack(cfg.HTTPTimeout)
	stack = append(stack, extra...)
	return negroni.New(stack...)
}

// CustomMiddlewareForTest is a JSON response middleware used in integration tests.
func CustomMiddlewareForTest(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	m := map[string]string{"msg": "Calling custom middleware"}
	b, _ := json.Marshal(m)
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
	next(w, r)
}
