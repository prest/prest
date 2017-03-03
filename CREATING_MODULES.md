
# Creating custom modules

## In order to create custom modules for pREST you need extends the router and register the custom new routes.


## Example:


```
package main

import (
	"net/http"
	"log"

	"github.com/nuveo/prest/cmd"
	"github.com/nuveo/prest/config/router"
	"github.com/nuveo/prest/config/middlewares"
	"github.com/urfave/negroni"
)

func main() {
	// Reorder middlewares
	middlewares.MiddlewareStack = []negroni.Handler{
		negroni.Handler(negroni.NewRecovery()),
		negroni.Handler(negroni.NewRecovery()),
		negroni.Handler(negroni.HandlerFunc(CustomMiddleware)),
	}

	// Get pREST app
	n := middlewares.GetApp()

	// Get pPREST router
	r := router.Get()

	// Register custom routes
	r.HandleFunc("/ping", Pong).Methods("GET")

	// Call pREST cmd
	cmd.Execute()
}

func Pong(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Pong!"))
	return
}

func CustomMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	log.Println("Calling custom middleware")
	next(w, r)
}
```