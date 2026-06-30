package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestAccessControl_Denied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().TablePermissions("test", "read", "").Return(false)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := AccessControl(perms)
	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req, next.ServeHTTP)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "authorization required")
}

func TestAccessControl_Allowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().TablePermissions("test", "read", "").Return(true)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := AccessControl(perms)
	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req, next.ServeHTTP)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAccessControl_SkipsNonTablePaths(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := AccessControl(perms)
	req := httptest.NewRequest(http.MethodGet, "/databases", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req, next.ServeHTTP)

	require.True(t, called)
}
