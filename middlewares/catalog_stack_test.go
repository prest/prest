package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
)

func serveCatalogStack(t *testing.T, stack *CatalogStack, req *http.Request) (*httptest.ResponseRecorder, bool) {
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

func catalogAuthConf() *config.Prest {
	return &config.Prest{AuthEnabled: true, JWTKey: testJWTHS256Key, JWTAlgo: "HS256"}
}

// TestNewCatalogStack_RequiresToken guards GHSA-fhf5-m388-4w4p: with auth
// enabled the catalog routes must reject a caller with no token.
func TestNewCatalogStack_RequiresToken(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	perms := mockgen.NewMockPermissionsChecker(ctrl)

	stack := NewCatalogStack(catalogAuthConf(), perms)
	require.Len(t, stack.Handlers(), 2)

	for _, path := range []string{"/databases", "/schemas", "/tables", "/prest-test/public", "/show/prest-test/public/test"} {
		rec, called := serveCatalogStack(t, stack, httptest.NewRequest(http.MethodGet, path, nil))
		require.False(t, called, path)
		require.Equal(t, http.StatusUnauthorized, rec.Code, path)
		require.Contains(t, rec.Body.String(), ErrAuthIsEmpty.Error(), path)
	}
}

func TestNewCatalogStack_ValidTokenListings(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	// Listing paths are not table paths, so the table ACL is never consulted.
	perms := mockgen.NewMockPermissionsChecker(ctrl)

	stack := NewCatalogStack(catalogAuthConf(), perms)
	token := signTestJWT(t, testJWTHS256Key, validClaims())
	for _, path := range []string{"/databases", "/schemas", "/tables", "/prest-test/public"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec, called := serveCatalogStack(t, stack, req)
		require.True(t, called, path)
		require.Equal(t, http.StatusOK, rec.Code, path)
	}
}

func TestNewCatalogStack_ShowAppliesTableACL(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().TablePermissions("prest-test", "public", "prest_users", "read", "alice").Return(false)
	perms.EXPECT().TablePermissions("prest-test", "public", "test", "read", "alice").Return(true)

	stack := NewCatalogStack(catalogAuthConf(), perms)
	token := signTestJWT(t, testJWTHS256Key, validClaims())

	// A table the ACL denies cannot be described, even with a valid token.
	req := httptest.NewRequest(http.MethodGet, "/show/prest-test/public/prest_users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec, called := serveCatalogStack(t, stack, req)
	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// A permitted table is described.
	req = httptest.NewRequest(http.MethodGet, "/show/prest-test/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec, called = serveCatalogStack(t, stack, req)
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestNewCatalogStack_AuthDisabledPassesThrough(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	perms := mockgen.NewMockPermissionsChecker(ctrl)

	stack := NewCatalogStack(&config.Prest{AuthEnabled: false}, perms)
	rec, called := serveCatalogStack(t, stack, httptest.NewRequest(http.MethodGet, "/databases", nil))
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}
