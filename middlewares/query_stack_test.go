package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/middlewares/statements"
	"github.com/stretchr/testify/require"
)

type stubScriptPerms struct {
	allow bool
}

func (s stubScriptPerms) ScriptPermissions(_ context.Context, _, _, _, _, _ string) bool {
	return s.allow
}

type capturingScriptPerms struct {
	allow                                    bool
	db, location, name, permission, userName string
}

func (s *capturingScriptPerms) ScriptPermissions(_ context.Context, db, location, name, permission, userName string) bool {
	s.db, s.location, s.name, s.permission, s.userName = db, location, name, permission, userName
	return s.allow
}

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

func TestScriptPermsFromAdapter_DenyAllWhenAbsent(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := ScriptPermsFromAdapter(mockgen.NewMockAdapter(ctrl))
	require.False(t, perms.ScriptPermissions(context.Background(), "db", "loc", "script", "read", "user"))
}

type adapterWithScriptPerms struct {
	*mockgen.MockAdapter
	stubScriptPerms
}

func TestScriptPermsFromAdapter_UsesAdapterChecker(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := ScriptPermsFromAdapter(&adapterWithScriptPerms{
		MockAdapter:     mockgen.NewMockAdapter(ctrl),
		stubScriptPerms: stubScriptPerms{allow: true},
	})
	require.True(t, perms.ScriptPermissions(context.Background(), "db", "loc", "script", "read", "user"))
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

func TestNewQueryStack_AuthOnly(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		AuthEnabled: true,
		JWTKey:      "secret",
		QueriesConf: config.QueriesConf{Restrict: false},
	}
	stack := NewQueryStack(cfg, stubScriptPerms{allow: true})
	require.Len(t, stack.Handlers(), 1)
}

func TestNewQueryStack_RestrictOnly(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		AuthEnabled: false,
		QueriesConf: config.QueriesConf{Restrict: true},
	}
	stack := NewQueryStack(cfg, stubScriptPerms{allow: true})
	require.Len(t, stack.Handlers(), 2)
}

func TestNewQueryStack_Neither(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		AuthEnabled: false,
		QueriesConf: config.QueriesConf{Restrict: false},
	}
	stack := NewQueryStack(cfg, stubScriptPerms{allow: true})
	require.Len(t, stack.Handlers(), 0)
}

func TestQueryStack_Handlers(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		AuthEnabled: true,
		JWTKey:      "secret",
		QueriesConf: config.QueriesConf{Restrict: true},
	}
	stack := NewQueryStack(cfg, stubScriptPerms{allow: true})
	handlers := stack.Handlers()
	require.Len(t, handlers, 2)
	require.Equal(t, handlers, stack.Handlers())
}

func TestNewAdminQueryStack(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		AuthEnabled: true,
		JWTKey:      "secret",
		QueriesConf: config.QueriesConf{RegisterAdmins: []string{"admin"}},
	}
	stack := NewAdminQueryStack(cfg)
	require.Len(t, stack.Handlers(), 2)
}

func TestAdminQueryStack_Handlers(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		AuthEnabled: true,
		JWTKey:      "secret",
		QueriesConf: config.QueriesConf{RegisterAdmins: []string{"admin"}},
	}
	stack := NewAdminQueryStack(cfg)
	handlers := stack.Handlers()
	require.Len(t, handlers, 2)
	require.Equal(t, handlers, stack.Handlers())
}

func TestScriptAccessControl_NoPermissionForMethod(t *testing.T) {
	t.Parallel()

	perms := &capturingScriptPerms{allow: false}
	handler := ScriptAccessControl(perms)
	req := httptest.NewRequest(http.MethodOptions, "/_QUERIES/fulltable/get_all", nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "fulltable", "script": "get_all"})
	rec := httptest.NewRecorder()

	called := false
	handler.ServeHTTP(rec, req, func(rw http.ResponseWriter, r *http.Request) { called = true })
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestScriptAccessControl_MissingUser(t *testing.T) {
	t.Parallel()

	perms := &capturingScriptPerms{allow: false}
	handler := ScriptAccessControl(perms)
	req := httptest.NewRequest(http.MethodGet, "/_QUERIES/fulltable/get_all", nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "fulltable", "script": "get_all"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call next") })
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Empty(t, perms.userName)
}

func TestScriptAccessControl_WrongUserType(t *testing.T) {
	t.Parallel()

	perms := &capturingScriptPerms{allow: true}
	handler := ScriptAccessControl(perms)
	req := httptest.NewRequest(http.MethodGet, "/_QUERIES/fulltable/get_all", nil)
	req = mux.SetURLVars(req, map[string]string{"queriesLocation": "fulltable", "script": "get_all"})
	ctx := context.WithValue(req.Context(), pctx.UserInfoKey, "not-a-user")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	called := false
	handler.ServeHTTP(rec, req, func(rw http.ResponseWriter, r *http.Request) { called = true })
	require.True(t, called)
	require.Empty(t, perms.userName)
}

func TestScriptAccessControl_WithDatabaseVar(t *testing.T) {
	t.Parallel()

	perms := &capturingScriptPerms{allow: true}
	handler := ScriptAccessControl(perms)
	req := httptest.NewRequest(http.MethodPost, "/_QUERIES/mydb/fulltable/get_all", nil)
	req = mux.SetURLVars(req, map[string]string{
		"database":        "mydb",
		"queriesLocation": "fulltable",
		"script":          "get_all",
	})
	ctx := context.WithValue(req.Context(), pctx.UserInfoKey, auth.User{Username: "alice"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	called := false
	handler.ServeHTTP(rec, req, func(rw http.ResponseWriter, r *http.Request) { called = true })
	require.True(t, called)
	require.Equal(t, "mydb", perms.db)
	require.Equal(t, "fulltable", perms.location)
	require.Equal(t, "get_all", perms.name)
	require.Equal(t, statements.WRITE, perms.permission)
	require.Equal(t, "alice", perms.userName)
}

func TestRegisterAdminGuard_NoUser(t *testing.T) {
	t.Parallel()

	handler := RegisterAdminGuard([]string{"admin"})
	req := httptest.NewRequest(http.MethodPost, "/_QUERIES/registry", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call next") })
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRegisterAdminGuard_WrongUserType(t *testing.T) {
	t.Parallel()

	handler := RegisterAdminGuard([]string{"admin"})
	req := httptest.NewRequest(http.MethodPost, "/_QUERIES/registry", nil)
	ctx := context.WithValue(req.Context(), pctx.UserInfoKey, 42)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call next") })
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRegisterAdminGuard_TrimsAdminNames(t *testing.T) {
	t.Parallel()

	called := false
	handler := RegisterAdminGuard([]string{" admin "})
	req := httptest.NewRequest(http.MethodPost, "/_QUERIES/registry", nil)
	ctx := context.WithValue(req.Context(), pctx.UserInfoKey, auth.User{Username: "admin"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req, func(rw http.ResponseWriter, r *http.Request) { called = true })
	require.True(t, called)
}

func TestAdminUsernameFromContext(t *testing.T) {
	t.Parallel()

	t.Run("nil context value", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, AdminUsernameFromContext(context.Background()))
	})

	t.Run("wrong type", func(t *testing.T) {
		t.Parallel()
		ctx := context.WithValue(context.Background(), pctx.UserInfoKey, "not-a-user")
		require.Empty(t, AdminUsernameFromContext(ctx))
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctx := context.WithValue(context.Background(), pctx.UserInfoKey, auth.User{Username: "admin"})
		require.Equal(t, "admin", AdminUsernameFromContext(ctx))
	})
}
