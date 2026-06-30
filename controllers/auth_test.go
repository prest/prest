package controllers

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/stretchr/testify/require"
	jose "gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

func testAuthHandler() *AuthHandler {
	return NewAuthHandler(nil, AuthConfig{
		Schema:   "public",
		Table:    "prest_users",
		Username: "username",
		Password: "password",
		Encrypt:  "MD5",
	})
}

func testAuthConfig() AuthConfig {
	return AuthConfig{
		AuthType: "body",
		JWTKey:   "test-secret",
		Schema:   "public",
		Table:    "prest_users",
		Username: "username",
		Password: "password",
		Encrypt:  "MD5",
	}
}

func md5Hex(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func Test_getSelectQuery(t *testing.T) {
	expected := "SELECT * FROM public.prest_users WHERE username=$1 AND password=$2 LIMIT 1"
	query := testAuthHandler().selectQuery()

	if query != expected {
		t.Errorf("expected query: %s, got: %s", expected, query)
	}
}

func Test_encrypt(t *testing.T) {
	h := testAuthHandler()
	pwd := "123456"
	enc := h.encrypt(pwd)

	md5Enc := fmt.Sprintf("%x", md5.Sum([]byte(pwd)))
	if enc != md5Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, md5Enc)
	}

	h.cfg.Encrypt = "SHA1"
	enc = h.encrypt(pwd)

	sha1Enc := fmt.Sprintf("%x", sha1.Sum([]byte(pwd)))
	if enc != sha1Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, sha1Enc)
	}
}

func Test_encrypt_unknownAlgorithm(t *testing.T) {
	h := testAuthHandler()
	h.cfg.Encrypt = "PLAINTEXT"
	require.Empty(t, h.encrypt("secret"))
}

func TestAuthHandler_Login_BodySuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)
	expectedQuery := testAuthHandler().selectQuery()

	executor.EXPECT().
		Query(expectedQuery, "alice", md5Hex("secret")).
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		u, ok := dest.(*auth.User)
		require.True(t, ok)
		*u = auth.User{ID: 1, Username: "alice", Name: "Alice"}
		return 1, nil
	})

	h := NewAuthHandler(executor, testAuthConfig())
	body := bytes.NewBufferString(`{"username":"Alice","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth", body)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp Response
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.NotEmpty(t, resp.Token)
	require.Equal(t, "alice", resp.LoggedUser.(map[string]interface{})["username"])

	parsed, err := jwt.ParseSigned(resp.Token)
	require.NoError(t, err)
	var claims auth.Claims
	require.NoError(t, parsed.Claims([]byte("test-secret"), &claims))
	require.Equal(t, "alice", claims.UserInfo.Username)
}

func TestAuthHandler_Login_BodyUserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)

	executor.EXPECT().
		Query(gomock.Any(), "nobody", gomock.Any()).
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).Return(0, nil)

	h := NewAuthHandler(executor, testAuthConfig())
	body := bytes.NewBufferString(`{"username":"nobody","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth", body)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), unf)
}

func TestAuthHandler_Login_BodyQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)

	executor.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Return(sc)
	sc.EXPECT().Err().Return(fmt.Errorf("db down")).Times(2)

	h := NewAuthHandler(executor, testAuthConfig())
	body := bytes.NewBufferString(`{"username":"alice","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth", body)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "db down")
}

func TestAuthHandler_Login_BasicMissingCredentials(t *testing.T) {
	h := NewAuthHandler(nil, AuthConfig{AuthType: "basic"})
	req := httptest.NewRequest(http.MethodPost, "/auth", nil)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), unf)
}

func TestAuthHandler_Login_BasicSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)

	executor.EXPECT().
		Query(gomock.Any(), "bob", md5Hex("pass")).
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		u := dest.(*auth.User)
		*u = auth.User{ID: 2, Username: "bob"}
		return 1, nil
	})

	cfg := testAuthConfig()
	cfg.AuthType = "basic"
	h := NewAuthHandler(executor, cfg)

	req := httptest.NewRequest(http.MethodPost, "/auth", nil)
	req.SetBasicAuth("Bob", "pass")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp Response
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.NotEmpty(t, resp.Token)
}

func TestAuthHandler_token(t *testing.T) {
	h := NewAuthHandler(nil, AuthConfig{JWTKey: "signing-key"})
	user := auth.User{ID: 9, Username: "jwt-user", Name: "JWT User"}

	token, err := h.token(user)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	parsed, err := jwt.ParseSigned(token)
	require.NoError(t, err)

	var claims auth.Claims
	require.NoError(t, parsed.Claims([]byte("signing-key"), &claims))
	require.Equal(t, user.ID, claims.UserInfo.ID)
	require.Equal(t, user.Username, claims.UserInfo.Username)
	require.NotNil(t, claims.Expiry)
	require.NotNil(t, claims.NotBefore)

	sig, err := jose.ParseSigned(token)
	require.NoError(t, err)
	require.Equal(t, "HS256", string(sig.Signatures[0].Header.Algorithm))
}

func TestAuthHandler_basicPasswordCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)
	h := NewAuthHandler(executor, testAuthConfig())

	executor.EXPECT().
		Query(h.selectQuery(), "carol", md5Hex("pw")).
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		u := dest.(*auth.User)
		*u = auth.User{ID: 3, Username: "carol"}
		return 1, nil
	})

	user, err := h.basicPasswordCheck("carol", "pw")
	require.NoError(t, err)
	require.Equal(t, "carol", user.Username)
}

func TestToken(t *testing.T) {
	config.PrestConf = &config.Prest{JWTKey: "legacy-key"}
	t.Cleanup(func() { config.PrestConf = nil })

	user := auth.User{ID: 7, Username: "legacy"}
	token, err := Token(user)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	parsed, err := jwt.ParseSigned(token)
	require.NoError(t, err)
	var claims auth.Claims
	require.NoError(t, parsed.Claims([]byte("legacy-key"), &claims))
	require.Equal(t, user.Username, claims.UserInfo.Username)
}
