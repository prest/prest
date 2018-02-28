---
title: "Extending (framework)"
date: 2017-08-30T19:07:05-03:00
weight: 15
menu: main
chapter: true
---

pREST was developed with the possibility of using it as a web framework, being able to use it based on its API, you create new endpoints and place middleware, adapting the pREST to your need.

### Sample Hello World

In order to create custom modules for pREST you need extends the router and register the custom new routes.

```go
package main

import (
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
	middlewares.GetApp()

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
```