package middlewares

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"

	"strings"

	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/prest/config"
	"github.com/urfave/negroni"
)

// HandlerSet add content type header
func HandlerSet() negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		format := r.URL.Query().Get("_renderer")
		recorder := httptest.NewRecorder()
		negroniResp := negroni.NewResponseWriter(recorder)
		next(negroniResp, r)
		renderFormat(w, recorder, format)
	})
}

// AccessControl is a middleware to handle permissions on tables in pREST
func AccessControl() negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
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

		if config.PrestConf.Adapter.TablePermissions(mapPath["table"], permission) {
			next(rw, rq)
			return
		}

		err := fmt.Errorf("required authorization to table %s", mapPath["table"])
		http.Error(rw, err.Error(), http.StatusUnauthorized)
	})
}

// JwtMiddleware check if actual request have JWT
func JwtMiddleware(key string, algo string) negroni.Handler {
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(key), nil
		},
		SigningMethod: jwt.GetSigningMethod(algo),
	})
	return negroni.HandlerFunc(jwtMiddleware.HandlerWithNext)
}

// Cors middleware
func Cors(origin []string, headers []string) negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		w.Header().Set(headerAllowOrigin, strings.Join(origin, ","))
		w.Header().Set(headerAllowCredentials, strconv.FormatBool(true))
		if r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set(headerAllowMethods, strings.Join(defaultAllowMethods, ","))
			w.Header().Set(headerAllowHeaders, strings.Join(headers, ","))
			if allowed := checkCors(r, origin); !allowed {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	})
}
