package middlewares

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers/auth"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
	jose "gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

func validClaims() auth.Claims {
	return auth.Claims{
		UserInfo:  auth.User{ID: 1, Username: "alice"},
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
}

func signTestJWT(t *testing.T, key string, claims auth.Claims) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: []byte(key)},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)
	token, err := jwt.Signed(sig).Claims(claims).CompactSerialize()
	require.NoError(t, err)
	return token
}

func serveMiddleware(h negroni.Handler, req *http.Request) (*httptest.ResponseRecorder, bool) {
	rec := httptest.NewRecorder()
	called := false
	h.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) {
		called = true
	})
	return rec, called
}

func TestHandlerSet_JSONResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	HandlerSet().ServeHTTP(rec, req, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), `"ok":true`)
}

func TestHandlerSet_ErrorJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	HandlerSet().ServeHTTP(rec, req, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"error": "bad request"`)
}

func TestHandlerSet_XMLRenderer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?_renderer=xml", nil)
	rec := httptest.NewRecorder()

	HandlerSet().ServeHTTP(rec, req, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"name":"prest"}`))
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/xml", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), "<objects>")
	require.Contains(t, rec.Body.String(), "prest")
}

func TestSetTimeoutToContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	var timeout int
	SetTimeoutToContext(42).ServeHTTP(httptest.NewRecorder(), req, func(_ http.ResponseWriter, r *http.Request) {
		timeout, _ = r.Context().Value(pctx.HTTPTimeoutKey).(int)
	})

	require.Equal(t, 42, timeout)
}

func TestSetTimeoutToContext_DefaultZero(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	var ctx context.Context
	SetTimeoutToContext(0).ServeHTTP(httptest.NewRecorder(), req, func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})

	_, ok := ctx.Value(pctx.HTTPTimeoutKey).(int)
	require.True(t, ok)
}

func TestAuthMiddleware_AuthDisabled(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(AuthMiddleware(AuthSettings{Enabled: false}), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_WhitelistedURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec, called := serveMiddleware(AuthMiddleware(AuthSettings{
		Enabled:      true,
		JWTWhiteList: []string{`\/auth`},
	}), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthMiddleware_EmptyToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(AuthMiddleware(AuthSettings{Enabled: true}), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrAuthIsEmpty.Error())
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	token := signTestJWT(t, "secret", validClaims())
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	var user auth.User
	rec := httptest.NewRecorder()
	AuthMiddleware(AuthSettings{Enabled: true, JWTKey: "secret"}).ServeHTTP(rec, req, func(_ http.ResponseWriter, r *http.Request) {
		u, ok := r.Context().Value(pctx.UserInfoKey).(auth.User)
		require.True(t, ok)
		user = u
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "alice", user.Username)
}

func TestAuthMiddleware_EmptyKeyRejected(t *testing.T) {
	token := signTestJWT(t, "", validClaims())
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec, called := serveMiddleware(AuthMiddleware(AuthSettings{Enabled: true, JWTKey: ""}), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrJWTEmptyKey.Error())
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	claims := auth.Claims{
		NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}
	token := signTestJWT(t, "secret", claims)
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec, called := serveMiddleware(AuthMiddleware(AuthSettings{Enabled: true, JWTKey: "secret"}), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrJWTValidate.Error())
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")

	rec, called := serveMiddleware(AuthMiddleware(AuthSettings{Enabled: true, JWTKey: "secret"}), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrJWTParseFail.Error())
}

func TestAuthMiddleware_WrongSigningKey(t *testing.T) {
	token := signTestJWT(t, "other", validClaims())
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec, called := serveMiddleware(AuthMiddleware(AuthSettings{Enabled: true, JWTKey: "secret"}), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestJwtAlgo(t *testing.T) {
	require.Equal(t, jose.HS256, jwtAlgo("HS256"))
	require.Equal(t, jose.HS512, jwtAlgo("HS512"))
	require.Equal(t, jose.RS256, jwtAlgo("RS256"))
	require.Equal(t, jose.EdDSA, jwtAlgo("EdDSA"))
	require.Equal(t, jose.HS256, jwtAlgo("unknown"))
}

func TestAccessControl_Denied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().TablePermissions("prest-test", "public", "test", "read", "").Return(false)

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
	perms.EXPECT().TablePermissions("prest-test", "public", "test", "read", "").Return(true)

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

func TestAccessControl_SkipsNonPermissionMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := AccessControl(perms)
	req := httptest.NewRequest(http.MethodOptions, "/prest-test/public/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req, next.ServeHTTP)

	require.True(t, called)
}

func TestAccessControl_PassesUsername(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().TablePermissions("prest-test", "public", "test", "read", "bob").Return(true)

	handler := AccessControl(perms)
	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	req = req.WithContext(withUser(req.Context(), auth.User{Username: "bob"}))

	called := false
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) {
		called = true
	})

	require.True(t, called)
}

func withUser(ctx context.Context, user auth.User) context.Context {
	return context.WithValue(ctx, pctx.UserInfoKey, user)
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

func TestJwtMiddleware_WhitelistedURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	rec, called := serveMiddleware(JwtMiddleware("secret", "", "HS256", []string{`\/auth`}), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestJwtMiddleware_ValidHMACKey(t *testing.T) {
	token := signTestJWT(t, "secret", validClaims())
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec, called := serveMiddleware(JwtMiddleware("secret", "", "HS256", nil), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestJwtMiddleware_EmptyToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(JwtMiddleware("secret", "", "HS256", nil), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrAuthIsEmpty.Error())
}

func TestJwtMiddleware_InvalidToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer bad-token")

	rec, called := serveMiddleware(JwtMiddleware("secret", "", "HS256", nil), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrJWTParseFail.Error())
}

func TestJwtMiddleware_ExpiredClaims(t *testing.T) {
	claims := auth.Claims{
		NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}
	token := signTestJWT(t, "secret", claims)
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec, called := serveMiddleware(JwtMiddleware("secret", "", "HS256", nil), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrJWTValidate.Error())
}

func TestJwtMiddleware_WrongKey(t *testing.T) {
	token := signTestJWT(t, "other", validClaims())
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec, called := serveMiddleware(JwtMiddleware("secret", "", "HS256", nil), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrJWTValidate.Error())
}

func TestCors_PreflightAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Origin", "https://example.com")

	rec := httptest.NewRecorder()
	Cors([]string{"https://example.com"}, []string{"Authorization"}).ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called for OPTIONS preflight")
	})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get(headerAllowMethods), "POST")
	require.Contains(t, rec.Header().Get(headerAllowHeaders), "Authorization")
}

func TestCors_PreflightForbiddenOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Origin", "https://evil.com")

	rec := httptest.NewRecorder()
	Cors([]string{"https://example.com"}, nil).ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called when origin is forbidden")
	})

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCors_RegularRequestPassesThrough(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec, called := serveMiddleware(Cors([]string{"*"}, nil), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "*", rec.Header().Get(headerAllowOrigin))
}

func TestExposureMiddleware_DatabasesDenied(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/databases", nil)
	rec, called := serveMiddleware(ExposureMiddleware(config.ExposeConf{DatabaseListing: false}), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "unauthorized listing")
}

func TestExposureMiddleware_SchemasDenied(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/schemas", nil)
	rec, called := serveMiddleware(ExposureMiddleware(config.ExposeConf{SchemaListing: false}), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestExposureMiddleware_TablesDenied(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tables", nil)
	rec, called := serveMiddleware(ExposureMiddleware(config.ExposeConf{TableListing: false}), req)

	require.False(t, called)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestExposureMiddleware_Allowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/databases", nil)
	rec, called := serveMiddleware(ExposureMiddleware(config.ExposeConf{
		DatabaseListing: true,
		SchemaListing:   true,
		TableListing:    true,
	}), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestExposureMiddleware_NonListingPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	rec, called := serveMiddleware(ExposureMiddleware(config.ExposeConf{}), req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestNewForTest_CustomMiddleware(t *testing.T) {
	cfg := &config.Prest{}
	n := NewForTest(cfg, negroni.Handler(negroni.HandlerFunc(CustomMiddlewareForTest)))

	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	n.UseHandler(r)

	rec := httptest.NewRecorder()
	n.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Contains(t, rec.Body.String(), "Calling custom middleware")
}

func appTestWithJwt(t *testing.T) (*negroni.Negroni, *config.Prest) {
	cfg, err := config.Load()
	require.NoError(t, err)
	n := New(cfg)
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")
	n.UseHandler(r)
	return n, cfg
}

func TestJWTClaimsOk(t *testing.T) {
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS512")
	config.Load()
	nd, cfg := appTestWithJwt(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	getToken := time.Now()
	expireToken := time.Now().Add(time.Minute * 2)

	// TODO: JWT any Algorithm support
	sig, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.HS256,
			Key:       []byte(cfg.JWTKey)},
		(&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)

	cl := auth.Claims{
		NotBefore: jwt.NewNumericDate(getToken),
		Expiry:    jwt.NewNumericDate(expireToken),
	}
	bearer, err := jwt.Signed(sig).Claims(cl).CompactSerialize()
	require.NoError(t, err)
	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respd.StatusCode)
}

func TestJWTClaimsNotOk(t *testing.T) {
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS256")
	config.Load()
	nd, cfg := appTestWithJwt(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	getToken := time.Now()
	expireToken := time.Now().Add(-1 * time.Minute)

	// TODO: JWT any Algorithm support
	sig, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.HS256,
			Key:       []byte(cfg.JWTKey)},
		(&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)

	cl := auth.Claims{
		NotBefore: jwt.NewNumericDate(getToken),
		Expiry:    jwt.NewNumericDate(expireToken),
	}
	bearer, err := jwt.Signed(sig).Claims(cl).CompactSerialize()
	require.NoError(t, err)

	req.Header.Add("authorization", bearer)

	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, respd.StatusCode)
}

// todo: Add unit test for other types of keys
func TestJWKSetRSAOk(t *testing.T) {
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")

	//generate a private key and a JWKS
	raw, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	key, err := jwk.FromRaw(raw)
	require.NoError(t, err)

	jwks_private := jwk.NewSet()
	jwks_private.AddKey(key)

	jwks, err := jwk.PublicSetOf(jwks_private)
	require.NoError(t, err)

	jwkSetJSON, err := json.Marshal(jwks)
	require.NoError(t, err)

	t.Setenv("PREST_JWT_JWKS", string(jwkSetJSON))

	config.Load()
	nd, _ := appTestWithJwt(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	//generate token with valid signature
	getToken := time.Now()
	expireToken := time.Now().Add(time.Minute * 2)

	sig, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       raw},
		(&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)

	cl := auth.Claims{
		NotBefore: jwt.NewNumericDate(getToken),
		Expiry:    jwt.NewNumericDate(expireToken),
	}
	bearer, err := jwt.Signed(sig).Claims(cl).CompactSerialize()
	require.NoError(t, err)
	req.Header.Add("authorization", bearer)

	//validate signature with JWKS
	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, respd.StatusCode)
}

func TestJWKSetRSANoKey(t *testing.T) {
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")

	//generate a private key and a JWKS
	raw, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	key, err := jwk.FromRaw(raw)
	require.NoError(t, err)

	jwks_private := jwk.NewSet()
	jwks_private.AddKey(key)

	jwks, err := jwk.PublicSetOf(jwks_private)
	require.NoError(t, err)

	jwkSetJSON, err := json.Marshal(jwks)
	require.NoError(t, err)

	t.Setenv("PREST_JWT_JWKS", string(jwkSetJSON))

	//Generate wrong key
	raw, err = rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	config.Load()
	nd, _ := appTestWithJwt(t)
	serverd := httptest.NewServer(nd)
	defer serverd.Close()

	req, err := http.NewRequest("GET", serverd.URL, nil)
	require.NoError(t, err)

	//generate token with valid signature
	getToken := time.Now()
	expireToken := time.Now().Add(time.Minute * 2)

	sig, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       raw},
		(&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)

	cl := auth.Claims{
		NotBefore: jwt.NewNumericDate(getToken),
		Expiry:    jwt.NewNumericDate(expireToken),
	}
	bearer, err := jwt.Signed(sig).Claims(cl).CompactSerialize()
	require.NoError(t, err)
	req.Header.Add("authorization", bearer)

	//validate signature with JWKS
	client := http.Client{}
	respd, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, respd.StatusCode)
}

// Regression coverage for GHSA-fj7v-859r-2fm4: an HS256 token signed with the
// empty HMAC key must be rejected when JwtMiddleware is configured without
// verification material. Before the fix the middleware called
// `tok.Claims([]byte(""), &out)` which jose's HMAC implementation accepts,
// granting access to any caller able to forge `HMAC-SHA256("", header.payload)`.
func TestJWTEmptyKeyRejectsForgedToken(t *testing.T) {
	mw := JwtMiddleware("", "", "HS256", nil)

	// Forge a token signed with the empty secret. NotBefore/Expiry are valid,
	// so the only thing that should reject this request is the empty-key guard.
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: []byte("")},
		(&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)

	cl := auth.Claims{
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	forged, err := jwt.Signed(sig).Claims(cl).CompactSerialize()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+forged)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called when the verification key is empty")
	})

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrJWTEmptyKey.Error())
}

// When a JWKS is provided but does not contain the kid from the bearer token,
// the middleware must fail closed. Before the fix the code only flagged this
// case when the rawkey happened to be a string — for the default []byte path it
// silently fell through to verifying with []byte(""), which is the same
// auth-bypass shape as GHSA-fj7v-859r-2fm4.
func TestJWTJWKSWithoutMatchingKidRejected(t *testing.T) {
	// Minimal JWKS containing one RSA key with kid="other".
	raw, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	key, err := jwk.FromRaw(raw)
	require.NoError(t, err)
	require.NoError(t, key.Set(jwk.KeyIDKey, "other"))

	set := jwk.NewSet()
	set.AddKey(key)
	pub, err := jwk.PublicSetOf(set)
	require.NoError(t, err)
	jwksJSON, err := json.Marshal(pub)
	require.NoError(t, err)

	mw := JwtMiddleware("", string(jwksJSON), "HS256", nil)

	// Forge a token whose kid does not match anything in the JWKS.
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: []byte("")},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "missing"))
	require.NoError(t, err)
	cl := auth.Claims{
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		Expiry:    jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	forged, err := jwt.Signed(sig).Claims(cl).CompactSerialize()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/prest/public/test", nil)
	req.Header.Set("Authorization", "Bearer "+forged)
	rec := httptest.NewRecorder()

	mw.ServeHTTP(rec, req, func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler should not be called when the kid is not in the JWKS")
	})

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), ErrJWKSetKeyNotFound.Error())
}

func TestValidate(t *testing.T) {
	type args struct {
		c auth.Claims
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "validate token expiry&not before time",
			args:    args{auth.Claims{NotBefore: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), Expiry: jwt.NewNumericDate(time.Now().Add(1 * time.Hour))}},
			wantErr: false,
		},
		{
			name:    "validate token not before time",
			args:    args{auth.Claims{NotBefore: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)), Expiry: jwt.NewNumericDate(time.Now().Add(1 * time.Hour))}},
			wantErr: true,
		},
		{
			name:    "validate token expiry time",
			args:    args{auth.Claims{NotBefore: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), Expiry: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.args.c); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
