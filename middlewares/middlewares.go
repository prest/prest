package middlewares

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"

	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/urfave/negroni/v3"
	jose "gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	jsonErrFormat        = `{"error": "%s"}`
	ErrJWTParseFail      = errors.New("failed JWT token parser")
	ErrJWTValidate       = errors.New("failed JWT claims validated")
	ErrAuthRequired      = errors.New("authorization required")
	ErrAuthIsEmpty       = errors.New("authorization token is empty")
	ErrJWKSetParse       = errors.New("failed to parse JWKSet JSON string")
	ErrJWKSetCreate      = errors.New("failed to create public key")
	ErrJWKSetKeyNotFound = errors.New("the token's key was not found in the JWKS")
	// ErrJWTEmptyKey is returned when the middleware would otherwise validate a
	// bearer token using an empty HMAC key — that path lets clients forge
	// tokens against `[]byte("")`. We fail closed instead. See GHSA-fj7v-859r-2fm4.
	ErrJWTEmptyKey = errors.New("JWT verification key is empty; refusing to validate token")
)

// HandlerSet add content type header
func HandlerSet() negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		// Studio serves HTML/JS/CSS; do not force application/json.
		if r.URL.Path == "/_studio" || strings.HasPrefix(r.URL.Path, "/_studio/") {
			next(w, r)
			return
		}
		format := r.URL.Query().Get("_renderer")
		recorder := httptest.NewRecorder()
		negroniResp := negroni.NewResponseWriter(recorder)
		next(negroniResp, r)
		renderFormat(w, recorder, format)
	})
}

// AuthSettings holds auth middleware configuration.
type AuthSettings struct {
	Enabled      bool
	JWTKey       string
	JWTWhiteList []string
}

// SetTimeoutToContext adds the configured timeout in seconds to the request context.
func SetTimeoutToContext(timeout int) negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		next(rw, r.WithContext(context.WithValue(r.Context(), pctx.HTTPTimeoutKey, timeout))) // nolint
	})
}

// AuthMiddleware handle request token validation
func AuthMiddleware(settings AuthSettings) negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(r.URL.String(), settings.JWTWhiteList)
		if err != nil {
			http.Error(rw, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusInternalServerError)
			return
		}
		if settings.Enabled && !match {
			token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
			if token == "" {
				slog.Error("authorization token is empty")
				http.Error(rw, fmt.Sprintf(jsonErrFormat, ErrAuthIsEmpty.Error()), http.StatusUnauthorized)
				return
			}

			tok, err := jwt.ParseSigned(token)
			if err != nil {
				http.Error(rw, fmt.Sprintf(jsonErrFormat, ErrJWTParseFail.Error()), http.StatusUnauthorized)
				return
			}
			if settings.JWTKey == "" {
				slog.Error("JWT verification key is empty; refusing to validate token")
				http.Error(rw, fmt.Sprintf(jsonErrFormat, ErrJWTEmptyKey.Error()), http.StatusUnauthorized)
				return
			}
			claims := auth.Claims{}
			if err := tok.Claims([]byte(settings.JWTKey), &claims); err != nil {
				http.Error(rw, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusUnauthorized)
				return
			}
			if err := Validate(claims); err != nil {
				http.Error(rw, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, pctx.UserInfoKey, claims.UserInfo)
			r = r.WithContext(ctx)
		}

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
func AccessControl(perms adapters.PermissionsChecker) negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		mapPath := getVars(rq.URL.Path)
		if mapPath == nil {
			next(rw, rq)
			return
		}

		// Get user info from token
		ctx := rq.Context()
		userInfo := ctx.Value(pctx.UserInfoKey)
		var userName string
		if userInfo != nil {
			if user, ok := userInfo.(auth.User); ok {
				userName = user.Username
			}
		}

		permission := permissionByMethod(rq.Method)
		if permission == "" {
			next(rw, rq)
			return
		}

		if perms.TablePermissions(mapPath["database"], mapPath["schema"], mapPath["table"], permission, userName) {
			next(rw, rq)
			return
		}

		http.Error(rw, fmt.Sprintf(jsonErrFormat, ErrAuthRequired.Error()), http.StatusUnauthorized)
	})
}

// JwtMiddleware check if actual request have JWT
func JwtMiddleware(key string, JWKSet, _ string, whitelist []string) negroni.Handler {
	return negroni.HandlerFunc(func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		match, err := MatchURL(r.URL.String(), whitelist)
		if err != nil {
			http.Error(w, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusInternalServerError)
			return
		}
		if match {
			next(w, r)
			return
		}

		// extract authorization token
		token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
		if token == "" {
			http.Error(w, fmt.Sprintf(jsonErrFormat, ErrAuthIsEmpty.Error()), http.StatusUnauthorized)
			return
		}
		tok, err := jwt.ParseSigned(token)
		if err != nil {
			http.Error(w, fmt.Sprintf(jsonErrFormat, ErrJWTParseFail.Error()), http.StatusUnauthorized)
			return
		}
		out := auth.Claims{}
		var rawkey interface{} = []byte(key)
		// jwksMatched tracks whether a JWKS lookup actually populated rawkey
		// with a real key. We need this because the loop below silently leaves
		// rawkey as []byte("") when no kid matches — and HS256 happily
		// validates against an empty HMAC key, which would be an auth bypass.
		jwksMatched := false

		if JWKSet != "" {
			parsedJWKSet, err := jwk.ParseString(JWKSet)
			if err != nil {
				slog.Error("failed to parse JWKSet JSON string", "err", err)
				http.Error(w, fmt.Sprintf(jsonErrFormat, ErrJWKSetParse.Error()), http.StatusUnauthorized)
				return
			}
			for it := parsedJWKSet.Keys(context.Background()); it.Next(context.Background()); {
				pair := it.Pair()
				key := pair.Value.(jwk.Key)

				if key.KeyID() == tok.Headers[0].KeyID {
					if err := key.Raw(&rawkey); err != nil {
						slog.Error("failed to create public key", "err", err)
						http.Error(w, fmt.Sprintf(jsonErrFormat, ErrJWKSetCreate.Error()), http.StatusUnauthorized)
						return
					}
					jwksMatched = true
				}
			}
			if !jwksMatched {
				slog.Error("the token's key was not found in the JWKS")
				http.Error(w, fmt.Sprintf(jsonErrFormat, ErrJWKSetKeyNotFound.Error()), http.StatusUnauthorized)
				return
			}
		}

		// Defense-in-depth: if no JWKS resolved and the configured HMAC key is
		// empty, refuse the request instead of letting jose validate against
		// []byte(""). config.validateJWTConfig should already prevent this on
		// startup, but we keep the guard so a misconfigured runtime can't
		// silently degrade to "any token accepted". GHSA-fj7v-859r-2fm4.
		if !jwksMatched {
			if b, ok := rawkey.([]byte); ok && len(b) == 0 {
				slog.Error("JWT verification key is empty; refusing to validate token")
				http.Error(w, fmt.Sprintf(jsonErrFormat, ErrJWTEmptyKey.Error()), http.StatusUnauthorized)
				return
			}
		}

		if err := tok.Claims(rawkey, &out); err != nil {
			http.Error(w, fmt.Sprintf(jsonErrFormat, ErrJWTValidate.Error()), http.StatusUnauthorized)
			return
		}
		if err := Validate(out); err != nil {
			http.Error(w, fmt.Sprintf(jsonErrFormat, err.Error()), http.StatusUnauthorized)
			return
		}
		next(w, r)
	})
}

// Cors middleware
//
// Deprecated: we'll use github.com/rs/cors instead
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

func ExposureMiddleware(expose config.ExposeConf) negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		url := rq.URL.Path

		if strings.HasPrefix(url, "/databases") && !expose.DatabaseListing {
			http.Error(rw, fmt.Sprintf(jsonErrFormat, "unauthorized listing"), http.StatusUnauthorized)
			return
		}

		if strings.HasPrefix(url, "/tables") && !expose.TableListing {
			http.Error(rw, fmt.Sprintf(jsonErrFormat, "unauthorized listing"), http.StatusUnauthorized)
			return
		}

		if strings.HasPrefix(url, "/schemas") && !expose.SchemaListing {
			http.Error(rw, fmt.Sprintf(jsonErrFormat, "unauthorized listing"), http.StatusUnauthorized)
			return
		}

		next(rw, rq)
	})
}

// nolint
func jwtAlgo(algo string) jose.SignatureAlgorithm {
	switch algo {
	case "EdDSA":
		return jose.EdDSA
	case "HS256":
		return jose.HS256
	case "HS384":
		return jose.HS384
	case "HS512":
		return jose.HS512
	case "RS256":
		return jose.RS256
	case "RS384":
		return jose.RS384
	case "RS512":
		return jose.RS512
	case "ES256":
		return jose.ES256
	case "ES384":
		return jose.ES384
	case "ES512":
		return jose.ES512
	case "PS256":
		return jose.PS256
	case "PS384":
		return jose.PS384
	case "PS512":
		return jose.PS512
	default:
		return jose.HS256
	}
}
