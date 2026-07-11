package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"

	"github.com/gorilla/mux"
	"github.com/urfave/negroni/v3"
)

// QueryStack bundles middleware applied to custom query routes.
type QueryStack struct {
	handlers []negroni.Handler
}

// NewQueryStack builds the middleware chain for /_QUERIES execution routes.
func NewQueryStack(cfg *config.Prest, perms adapters.ScriptPermissionsChecker) *QueryStack {
	qc := cfg.QueriesConf
	handlers := make([]negroni.Handler, 0, 2)

	requireAuth := qc.Restrict || cfg.AuthEnabled
	if requireAuth {
		handlers = append(handlers, AuthMiddleware(AuthSettings{
			Enabled:      cfg.AuthEnabled,
			JWTKey:       cfg.JWTKey,
			JWTWhiteList: cfg.JWTWhiteList,
		}))
	}
	if qc.Restrict {
		handlers = append(handlers, ScriptAccessControl(perms))
	}
	return &QueryStack{handlers: handlers}
}

// Handlers returns the negroni handlers for this stack.
func (s *QueryStack) Handlers() []negroni.Handler {
	return s.handlers
}

// AdminQueryStack bundles middleware for query registration routes.
type AdminQueryStack struct {
	handlers []negroni.Handler
}

// NewAdminQueryStack builds auth + register-admin guard for registry routes.
func NewAdminQueryStack(cfg *config.Prest) *AdminQueryStack {
	return &AdminQueryStack{
		handlers: []negroni.Handler{
			AuthMiddleware(AuthSettings{
				Enabled:      cfg.AuthEnabled,
				JWTKey:       cfg.JWTKey,
				JWTWhiteList: cfg.JWTWhiteList,
			}),
			RegisterAdminGuard(cfg.QueriesConf.RegisterAdmins),
		},
	}
}

// Handlers returns the negroni handlers for this stack.
func (s *AdminQueryStack) Handlers() []negroni.Handler {
	return s.handlers
}

// ScriptAccessControl enforces per-script ACL from queries config.
func ScriptAccessControl(perms adapters.ScriptPermissionsChecker) negroni.Handler {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		vars := mux.Vars(rq)
		location := vars["queriesLocation"]
		name := vars["script"]
		database := vars["database"]
		if database == "" {
			database = ""
		}

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

		if perms.ScriptPermissions(ctx, database, location, name, permission, userName) {
			next(rw, rq)
			return
		}

		http.Error(rw, fmt.Sprintf(jsonErrFormat, ErrAuthRequired.Error()), http.StatusUnauthorized)
	})
}

// RegisterAdminGuard allows only configured admin usernames.
func RegisterAdminGuard(admins []string) negroni.Handler {
	allowed := make(map[string]struct{}, len(admins))
	for _, a := range admins {
		allowed[strings.TrimSpace(a)] = struct{}{}
	}
	return negroni.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request, next http.HandlerFunc) {
		ctx := rq.Context()
		userInfo := ctx.Value(pctx.UserInfoKey)
		if userInfo == nil {
			http.Error(rw, fmt.Sprintf(jsonErrFormat, ErrAuthRequired.Error()), http.StatusUnauthorized)
			return
		}
		user, ok := userInfo.(auth.User)
		if !ok {
			http.Error(rw, fmt.Sprintf(jsonErrFormat, ErrAuthRequired.Error()), http.StatusUnauthorized)
			return
		}
		if _, ok := allowed[user.Username]; !ok {
			http.Error(rw, fmt.Sprintf(jsonErrFormat, "registration not permitted"), http.StatusForbidden)
			return
		}
		next(rw, rq)
	})
}

// AdminUsernameFromContext returns the authenticated username for registry writes.
func AdminUsernameFromContext(ctx context.Context) string {
	userInfo := ctx.Value(pctx.UserInfoKey)
	if userInfo == nil {
		return ""
	}
	if user, ok := userInfo.(auth.User); ok {
		return user.Username
	}
	return ""
}
