---
title: "Custom middlewares"
date: 2017-02-28T16:05:05-03:00
weight: 15
chapter: true
---

Using the previous sample we can create our middleware as a function and use that with `GetApp()` ([godocs.org at prest/middlewares](https://godoc.org/github.com/prest/middlewares#GetApp)) that returns a `*negroni.Negroni` object.

```go
package main

import (
	"log"
	"net/http"

	"github.com/prest/adapters/postgres"
	"github.com/prest/cmd"
	"github.com/prest/config"
	"github.com/prest/config/router"
	"github.com/prest/middlewares"
)

func main() {
	config.Load()

	// Load Postgres Adapter
	postgres.Load()

	// Get pREST app
	n := middlewares.GetApp()

	// Register custom middleware
	n.UseFunc(CustomMiddleware)

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

### Reorder middlewares

It is possible to change the order of execution of middleware, for this we have the `middlewares.MiddlewareStack` ([godocs.org at prest/middlewares](https://godoc.org/github.com/prest/middlewares#pkg-variables)) that receives `negroni.Handler` where you pass an array with the new order.

```go
package main

import (
	"log"
	"net/http"


	"github.com/prest/adapters/postgres"
	"github.com/prest/cmd"
	"github.com/prest/config"
	"github.com/prest/config/router"
	"github.com/prest/middlewares"
	"github.com/urfave/negroni"
)

func main() {
	config.Load()
	// Load Postgres Adapter
	postgres.Load()
	// Reorder middlewares
	middlewares.MiddlewareStack = []negroni.Handler{
		negroni.Handler(negroni.NewRecovery()),
		negroni.Handler(negroni.NewLogger()),
		negroni.Handler(middlewares.HandlerSet()),
		negroni.Handler(negroni.HandlerFunc(CustomMiddleware)),
	}

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
