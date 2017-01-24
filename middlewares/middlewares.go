package middlewares

import (
	"fmt"
	"net/http"

	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/urfave/negroni"
)

// HandlerSet add content type header
func HandlerSet(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Header().Set("Content-Type", "application/json")
	next(w, r)
}

// AccessControl is a middleware to handle permissions on tables in pREST
func AccessControl(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
	mapPath := getVars(rq.URL.Path)
	if mapPath == nil {
		next(rw, rq)
		return
	}

	permission := permissionByMethod(rq.Method)
	if permission == "" {
		next(rw, rq)
		return
	}

	if postgres.TablePermissions(mapPath["table"], permission) {
		next(rw, rq)
		return
	}

	err := fmt.Errorf("required authorization to table %s", mapPath["table"])
	http.Error(rw, err.Error(), http.StatusUnauthorized)
}

// JwtMiddleware check if actual request have JWT
func JwtMiddleware(key string) negroni.Handler {
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(key), nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})
	return negroni.HandlerFunc(jwtMiddleware.HandlerWithNext)
}
