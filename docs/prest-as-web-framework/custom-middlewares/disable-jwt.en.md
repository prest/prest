---
title: "Disable JWT Middleware"
date: 2017-02-28T16:05:05-03:00
weight: 15
chapter: true
---

Using pREST as framework is common to need to do endpoint for **authentication**, and that endpoint cannot ask for **JWT** (header) because it will generate the token.

### Create sub routes without JWT?

```go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/prest/prest/adapters/postgres"
	"github.com/prest/prest/cmd"
	"github.com/prest/prest/config"
	"github.com/prest/prest/config/router"
	"github.com/prest/prest/middlewares"
	"github.com/urfave/negroni"
)

const authPrefix = "/auth"

// Body data structure used to receive request
type Body struct {
	Username string
	Password string
}

// Auth data structure used to return authentication token
type Auth struct {
	Token string
}

func main() {
	// start pREST config
	config.Load()

	// pREST Postgres
	postgres.Load()

	// pREST routes
	r := router.Get()

	// Common middleware this application
	commonMiddleware := negroni.New(
		negroni.NewRecovery(),
		negroni.NewLogger(),
	)

	// Auth routers
	authR := mux.NewRouter().PathPrefix(authPrefix).Subrouter().StrictSlash(true)
	authR.HandleFunc("", AuthHandler).Methods("POST")
	r.PathPrefix(authPrefix).Handler(commonMiddleware.With(
		negroni.Wrap(authR),
	))

	// pREST middlewares
	middlewares.MiddlewareStack = []negroni.Handler{}
	r.PathPrefix("/").Handler(commonMiddleware.With(
		negroni.Handler(middlewares.JwtMiddleware(config.PrestConf.JWTKey)),
	))

	// Call pREST cmd
	cmd.Execute()
}

// AuthHandler user authentication Handler
func AuthHandler(w http.ResponseWriter, r *http.Request) {
	body := Body{}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tokenString, err := tokenGenerate(body.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	auth := Auth{
		Token: tokenString,
	}
	w.WriteHeader(http.StatusOK)
	ret, _ := json.Marshal(auth)
	w.Write(ret)
}

// tokenGenerate return token JWT (simulating authentication)
func tokenGenerate(Username string) (signedToken string, err error) {
	// Create the Claims
	claims := &jwt.StandardClaims{
		ExpiresAt: time.Now().Add(time.Hour * 1).Unix(),
		Issuer:    Username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err = token.SignedString([]byte(config.PrestConf.JWTKey))
	return
}
```

#### Test request

```sh
curl -X POST -i -H "Content-Type: application/json" \
-d '{"username": "a", "password": "a"}' \
http://127.0.0.1:3000/auth
```
