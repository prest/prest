
# Creating custom modules

## In order to create custom modules for pREST you need extends the router and register the custom new routes.


## Example:


```
package main

import (
	"net/http"

	"github.com/nuveo/prest/cmd"
	"github.com/nuveo/prest/config"
)

func main() {
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
```