package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers/auth"
	"github.com/urfave/negroni"
	"gopkg.in/square/go-jose.v2/jwt"
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

// AuthMiddleware handle request token validation
func AuthMiddleware() negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(r.URL.String())
		if err != nil {
			http.Error(rw, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}
		if config.PrestConf.AuthEnabled && !match {
			// extract authorization token
			token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
			if token == "" {
				err := fmt.Errorf("authorization token is empty")
				http.Error(rw, err.Error(), http.StatusUnauthorized)
				return
			}

			tok, err := jwt.ParseSigned(token)
			if err != nil {
				http.Error(rw, fmt.Errorf("Failed JWT token parser").Error(), http.StatusUnauthorized)
				return
			}
			claims := auth.Claims{}
			if err := tok.Claims([]byte(config.PrestConf.JWTKey), &claims); err != nil {
				http.Error(rw, err.Error(), http.StatusUnauthorized)
				return
			}

			// pass user_info to the next handler
			ctx := r.Context()
			ctx = context.WithValue(ctx, "user_info", claims.UserInfo)
			r = r.WithContext(ctx)
		}

		// if auth isn't enabled
		next(rw, r)
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
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(r.URL.String())
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}
		if match {
			next(w, r)
			return
		}

		// extract authorization token
		token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
		if token == "" {
			err := fmt.Errorf("authorization token is empty")
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		tok, err := jwt.ParseSigned(token)
		if err != nil {
			http.Error(w, fmt.Errorf("Failed JWT token parser").Error(), http.StatusUnauthorized)
			return
		}
		out := auth.Claims{}
		if err := tok.Claims([]byte(key), &out); err != nil {
			http.Error(w, fmt.Errorf("Failed JWT claims validated").Error(), http.StatusUnauthorized)
			return
		}
		next(w, r)
	})
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
