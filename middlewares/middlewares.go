package middlewares

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers/auth"
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
			ts := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
			if ts == "" {
				err := fmt.Errorf("authorization token is empty")
				http.Error(rw, err.Error(), http.StatusForbidden)
				return
			}

			_, err := jwt.ParseWithClaims(ts, &auth.Claims{}, func(token *jwt.Token) (interface{}, error) {
				// verify token sign method
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}

				// parse token claims
				var claims *auth.Claims
				if v, ok := token.Claims.(*auth.Claims); ok {
					claims = v
				} else {
					return nil, fmt.Errorf("token invalid")
				}

				// pass user_info to the next handler
				ctx := r.Context()
				ctx = context.WithValue(ctx, "user_info", claims.UserInfo)
				r = r.WithContext(ctx)

				return []byte(config.PrestConf.JWTKey), nil
			})

			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadRequest)
				return
			}
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

var ErrNoPEMKey = errors.New("no RSA/EC/PUBLIC PEM data found")

func parsePEMKey(data []byte) (key interface{}, err error) {
	var block *pem.Block
	for len(data) > 0 {
		block, data = pem.Decode(data)
		if block == nil {
			return nil, ErrNoPEMKey
		}
		switch block.Type {
		case "RSA PRIVATE KEY":
			return x509.ParsePKCS1PrivateKey(block.Bytes)
		case "EC PRIVATE KEY":
			return x509.ParseECPrivateKey(block.Bytes)
		case "PUBLIC KEY":
			return x509.ParsePKIXPublicKey(block.Bytes)
		}
	}
	return
}

// JwtMiddleware check if actual request have JWT
func JwtMiddleware(key string, algo string) negroni.Handler {
	var parsedKey interface{}
	if strings.HasPrefix(algo, "HS") {
		parsedKey = []byte(key)
	} else if strings.HasPrefix(algo, "ES") || strings.HasPrefix(algo, "RS") {
		var err error
		parsedKey, err = parsePEMKey([]byte(key))
		if err != nil {
			panic(fmt.Sprintf("Cannot parse public key: %s", err))
		}
	} else {
		panic(fmt.Sprintf("Do not know how to parse key for algo %#v", algo))
	}
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return parsedKey, nil
		},
		SigningMethod: jwt.GetSigningMethod(algo),
	})

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
		err = jwtMiddleware.CheckJWT(w, r)
		if err != nil {
			log.Println("check jwt error", err.Error())
			w.Write([]byte(fmt.Sprintf(`{"error": "%v"}`, err.Error())))
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
