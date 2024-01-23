package middlewares

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/prest/prest/config"
	pctx "github.com/prest/prest/context"
	"github.com/prest/prest/controllers/auth"
	"github.com/urfave/negroni/v3"
	"gopkg.in/square/go-jose.v2/jwt"
)

type PermsFn func(table string, op string) bool

var (
	ErrJWTParseFail = errors.New("failed JWT token parser")
	ErrJWTValidate  = errors.New("failed JWT claims validated")
)

// HandlerSet add content type to the header response
// and set the response format to the requested format
func HandlerSet() negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		format := r.URL.Query().Get("_renderer")
		recorder := httptest.NewRecorder()
		negroniResp := negroni.NewResponseWriter(recorder)
		next(negroniResp, r)
		renderFormat(w, recorder, format)
	})
}

// SetTimeoutToContext adds the configured timeout in seconds to the request context
//
// By default it is 60 seconds, can be modified to a different value
func SetTimeoutToContext(timeout int) negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		next(rw, r.WithContext(context.WithValue(r.Context(), pctx.HTTPTimeoutKey, timeout))) // nolint
	})
}

// AuthMiddleware handles request token validation and user info extraction from token
//
// if token is valid, it will pass user_info to the next handler
//
// if token is invalid, it will return 401
//
// if token is not present, it will return 401
//
// if token is present but not valid, it will return 401
func AuthMiddleware(enabled bool, key string, ignoreList []string) negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(ignoreList, r.URL.String())
		if err != nil {
			http.Error(rw, fmt.Sprintf(`{"error": "%v"}`, err), http.StatusInternalServerError)
			return
		}
		if !enabled || match {
			next(rw, r)
			return
		}

		// extract authorization token
		token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
		if token == "" {
			err := fmt.Errorf("authorization token is empty")
			http.Error(rw, err.Error(), http.StatusUnauthorized)
			return
		}

		tok, err := jwt.ParseSigned(token)
		if err != nil {
			http.Error(rw, ErrJWTParseFail.Error(), http.StatusUnauthorized)
			return
		}
		claims := auth.Claims{}
		if err := tok.Claims([]byte(key), &claims); err != nil {
			http.Error(rw, err.Error(), http.StatusUnauthorized)
			return
		}
		if err := Validate(claims); err != nil {
			http.Error(rw, err.Error(), http.StatusUnauthorized)
			return
		}

		// pass user_info to the next handler
		ctx := r.Context()
		ctx = context.WithValue(ctx, pctx.UserInfoKey, claims.UserInfo)
		r = r.WithContext(ctx)

		next(rw, r)
	})
}

// Validate claims
func Validate(c auth.Claims) error {
	if c.Expiry != nil && time.Now().After(c.Expiry.Time()) {
		return ErrJWTValidate
	}
	if c.NotBefore != nil && time.Now().Before(c.NotBefore.Time()) {
		return ErrJWTValidate
	}
	return nil
}

// AccessControl is a middleware to handle permissions on tables in pREST
func AccessControl(permFnc PermsFn) negroni.Handler {
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

		if permFnc(mapPath["table"], permission) {
			next(rw, rq)
			return
		}

		err := fmt.Errorf("required authorization to table %s", mapPath["table"])
		http.Error(rw, err.Error(), http.StatusUnauthorized)
	})
}

// JwtMiddleware check if actual request have JWT token in header Authorization
// and validate it with JWTKey and JWTWhiteList
//
// if token is valid, it will pass user_info to the next handler
//
// if token is invalid, it will return 401
//
// if token is not present, it will return 401
//
// if token is present but not valid, it will return 401
func JwtMiddleware(key string, ignoreList []string) negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(ignoreList, r.URL.String())
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
			http.Error(w, ErrJWTParseFail.Error(), http.StatusUnauthorized)
			return
		}
		out := auth.Claims{}
		if err := tok.Claims([]byte(key), &out); err != nil {
			http.Error(w, ErrJWTValidate.Error(), http.StatusUnauthorized)
			return
		}
		if err := Validate(out); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		next(w, r)
	})
}

func ExposureMiddleware(cfg *config.ExposeConf) negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		url := rq.URL.Path

		if strings.HasPrefix(url, "/databases") && !cfg.DatabaseListing {
			http.Error(rw, "unauthorized listing", http.StatusUnauthorized)
			return
		}

		if strings.HasPrefix(url, "/tables") && !cfg.TableListing {
			http.Error(rw, "unauthorized listing", http.StatusUnauthorized)
			return
		}

		if strings.HasPrefix(url, "/schemas") && !cfg.SchemaListing {
			http.Error(rw, "unauthorized listing", http.StatusUnauthorized)
			return
		}

		next(rw, rq)
	})
}
