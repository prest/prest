
# Creating custom modules

## In order to create custom modules for pREST you need extends the router and register the custom new routes.


## Example:


```
package main

import (
	"net/http"
	"log"

	"github.com/nuveo/prest/cmd"
	"github.com/nuveo/prest/config"
)

func main() {
	// Get pREST app
	n := config.GetApp()

	// Register custom middleware
	n.Use(negroni.HandlerFunc(CustomMiddleware))

	// Get pPREST router
	r := config.GetRouter()

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