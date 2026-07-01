package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/cache"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/plugins"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
)

func serveCRUDStack(t *testing.T, stack *CRUDStack, req *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	called := false
	n := negroni.New()
	for _, h := range stack.Handlers() {
		n.Use(h)
	}
	n.UseHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	n.ServeHTTP(rec, req)
	return rec, called
}

func TestNewCRUDStack(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := mockgen.NewMockAdapter(ctrl)
	adapter.EXPECT().TablePermissions("test", "read", "").Return(true)

	cfg := &config.Prest{
		Adapter:     adapter,
		AuthEnabled: false,
		JWTAlgo:     "HS256",
		Cache:       cache.Config{Enabled: false},
	}
	stack := NewCRUDStack(cfg, plugins.New(cfg))
	require.Len(t, stack.Handlers(), 5)

	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	rec, called := serveCRUDStack(t, stack, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestNewCRUDStackWithPerms(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().TablePermissions("test", "read", "").Return(true)

	cfg := &config.Prest{
		AuthEnabled: false,
		JWTAlgo:     "HS256",
		Cache:       cache.Config{Enabled: false},
	}
	stack := NewCRUDStackWithPerms(cfg, plugins.New(cfg), perms)
	require.Len(t, stack.Handlers(), 5)

	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	rec, called := serveCRUDStack(t, stack, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}
