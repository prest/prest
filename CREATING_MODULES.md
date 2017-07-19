
# Creating custom modules

## In order to create custom modules for pREST you need extends the router and register the custom new routes.


## Example:


```go
package main

import (
	"log"
	"net/http"

	"github.com/nuveo/prest/cmd"
	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/config/middlewares"
	"github.com/nuveo/prest/config/router"
	"github.com/urfave/negroni"
)

func main() {
	config.Load()
	// Reorder middlewares
	middlewares.MiddlewareStack = []negroni.Handler{
		negroni.Handler(negroni.NewRecovery()),
		negroni.Handler(negroni.NewLogger()),
		negroni.Handler(negroni.HandlerFunc(CustomMiddleware)),
	}

	// Get pREST app
	n := middlewares.GetApp()

	// Rgister custom middleware
	n.Use(negroni.HandlerFunc(CustomMiddleware))

	// Get pPREST router
	r := router.Get()

	// Register custom routes
	r.HandleFunc("/ping", Pong).Methods("GET")

	// Call pREST cmd
	cmd.Execute()
}

func Pong(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Pong!"))
}

func CustomMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	log.Println("Calling custom middleware")
	next(w, r)
}
```
