package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/stretchr/testify/require"
)

type stubScriptPerms struct {
	allow bool
}

func (s stubScriptPerms) ScriptPermissions(_, _, _, _, _ string) bool { return s.allow }

func TestScriptAccessControl_Allowed(t *testing.T) {
	t.Parallel()

	called := false
	handler := ScriptAccessControl(stubScriptPerms{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/_QUERIES/fulltable/get_all", nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "fulltable", "script": "get_all"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req, func(rw http.ResponseWriter, r *http.Request) { called = true })
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestScriptAccessControl_Denied(t *testing.T) {
	t.Parallel()

	handler := ScriptAccessControl(stubScriptPerms{allow: false})
	req := httptest.NewRequest(http.MethodGet, "/_QUERIES/fulltable/get_all", nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "fulltable", "script": "get_all"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call next") })
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRegisterAdminGuard_Allowed(t *testing.T) {
	t.Parallel()

	called := false
	handler := RegisterAdminGuard([]string{"admin"})
	req := httptest.NewRequest(http.MethodPost, "/_QUERIES/registry", nil)
	ctx := context.WithValue(req.Context(), pctx.UserInfoKey, auth.User{Username: "admin"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req, func(rw http.ResponseWriter, r *http.Request) { called = true })
	require.True(t, called)
}

func TestRegisterAdminGuard_Forbidden(t *testing.T) {
	t.Parallel()

	handler := RegisterAdminGuard([]string{"admin"})
	req := httptest.NewRequest(http.MethodPost, "/_QUERIES/registry", nil)
	ctx := context.WithValue(req.Context(), pctx.UserInfoKey, auth.User{Username: "other"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call next") })
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestNewQueryStack_RestrictRequiresAuthMiddleware(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		AuthEnabled: true,
		JWTKey:      "secret",
		QueriesConf: config.QueriesConf{Restrict: true},
	}
	stack := NewQueryStack(cfg, stubScriptPerms{allow: true})
	require.Len(t, stack.Handlers(), 2)
}
