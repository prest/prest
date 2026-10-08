package controllers

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/internal/ident"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const testAuthJWTKey = "test-jwt-hmac-secret-key-32bytes"

func testAuthHandler() *AuthHandler {
	return NewAuthHandler(nil, pgDialect{}, AuthConfig{
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
		JWTKey:   testAuthJWTKey,
		Schema:   "public",
		Table:    "prest_users",
		Username: "username",
		Password: "password",
		Encrypt:  "MD5",
	}
}

// pgDialect and mysqlDialect stand in for the adapters' Dialect implementations.
type pgDialect struct{}

func (pgDialect) QuoteIdentifier(name string) (string, error) { return ident.Quote(name) }
func (pgDialect) Placeholder(n int) string                    { return fmt.Sprintf("$%d", n) }

type mysqlDialect struct{}

func (mysqlDialect) QuoteIdentifier(name string) (string, error) {
	if !ident.IsValid(name) {
		return "", fmt.Errorf("invalid identifier: %s", name)
	}
	return "`" + strings.ReplaceAll(name, ".", "`.`") + "`", nil
}
func (mysqlDialect) Placeholder(int) string { return "?" }

func mustQuery(t *testing.T) func(string, error) string {
	return func(q string, err error) string {
		t.Helper()
		require.NoError(t, err)
		return q
	}
}

func md5Hex(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))
}

func Test_getSelectQuery(t *testing.T) {
	t.Parallel()

	expected := `SELECT * FROM "public"."prest_users" WHERE "username"=$1 AND "password"=$2 LIMIT 1`
	query := mustQuery(t)(testAuthHandler().selectQuery())

	if query != expected {
		t.Errorf("expected query: %s, got: %s", expected, query)
	}
}

func Test_legacyDigest(t *testing.T) {
	t.Parallel()

	h := testAuthHandler()
	pwd := "123456"
	enc, err := h.legacyDigest(pwd)
	require.NoError(t, err)

	md5Enc := fmt.Sprintf("%x", md5.Sum([]byte(pwd)))
	if enc != md5Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, md5Enc)
	}

	h.cfg.Encrypt = "SHA1"
	enc, err = h.legacyDigest(pwd)
	require.NoError(t, err)

	sha1Enc := fmt.Sprintf("%x", sha1.Sum([]byte(pwd)))
	if enc != sha1Enc {
		t.Errorf("expected encrypted password to be: %s, got: %s", enc, sha1Enc)
	}
}

func Test_legacyDigest_unknownAlgorithm(t *testing.T) {
	t.Parallel()

	h := testAuthHandler()
	h.cfg.Encrypt = "PLAINTEXT"
	_, err := h.legacyDigest("secret")
	require.ErrorIs(t, err, ErrUnknownEncryptAlgorithm)
}

func TestHashPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("secret")
	require.NoError(t, err)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret")))
}

func TestAuthHandler_Login_BodySuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)
	expectedQuery := mustQuery(t)(testAuthHandler().selectQuery())

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

	h := NewAuthHandler(executor, pgDialect{}, testAuthConfig())
	body := bytes.NewBufferString(`{"username":"Alice","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth", body)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp Response
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.NotEmpty(t, resp.Token)
	require.Equal(t, "alice", resp.LoggedUser.(map[string]interface{})["username"])

	parsed, err := jwt.ParseSigned(resp.Token, []jose.SignatureAlgorithm{jose.HS256})
	require.NoError(t, err)
	var claims auth.Claims
	require.NoError(t, parsed.Claims([]byte(testAuthJWTKey), &claims))
	require.Equal(t, "alice", claims.UserInfo.Username)
}

func TestAuthHandler_Login_BodyUserNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)

	executor.EXPECT().
		Query(gomock.Any(), "nobody", gomock.Any()).
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).Return(0, nil)

	h := NewAuthHandler(executor, pgDialect{}, testAuthConfig())
	body := bytes.NewBufferString(`{"username":"nobody","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth", body)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), unf)
}

func TestAuthHandler_Login_BodyQueryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)

	executor.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Return(sc)
	sc.EXPECT().Err().Return(fmt.Errorf("db down")).Times(2)

	h := NewAuthHandler(executor, pgDialect{}, testAuthConfig())
	body := bytes.NewBufferString(`{"username":"alice","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth", body)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "db down")
}

func TestAuthHandler_Login_BasicMissingCredentials(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, pgDialect{}, AuthConfig{AuthType: "basic"})
	req := httptest.NewRequest(http.MethodPost, "/auth", nil)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), unf)
}

func TestAuthHandler_Login_BasicSuccess(t *testing.T) {
	t.Parallel()

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
	h := NewAuthHandler(executor, pgDialect{}, cfg)

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
	t.Parallel()

	h := NewAuthHandler(nil, pgDialect{}, AuthConfig{JWTKey: testAuthJWTKey})
	user := auth.User{ID: 9, Username: "jwt-user", Name: "JWT User"}

	token, err := h.token(user)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.HS256})
	require.NoError(t, err)

	var claims auth.Claims
	require.NoError(t, parsed.Claims([]byte(testAuthJWTKey), &claims))
	require.Equal(t, user.ID, claims.UserInfo.ID)
	require.Equal(t, user.Username, claims.UserInfo.Username)
	require.NotNil(t, claims.Expiry)
	require.NotNil(t, claims.NotBefore)

	sig, err := jose.ParseSigned(token, []jose.SignatureAlgorithm{jose.HS256})
	require.NoError(t, err)
	require.Equal(t, "HS256", string(sig.Signatures[0].Header.Algorithm))
}

func TestAuthHandler_tokenWrapsSerializeError(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, pgDialect{}, AuthConfig{JWTKey: "too-short"})
	_, err := h.token(auth.User{Username: "jwt-user"})

	require.ErrorIs(t, err, jose.ErrInvalidKeySize)
	require.ErrorContains(t, err, "serialize JWT")
}

func Test_getSelectQueryByUsername(t *testing.T) {
	t.Parallel()

	expected := `SELECT * FROM "public"."prest_users" WHERE "username"=$1 LIMIT 1`
	query := testAuthHandler()
	query.cfg.Encrypt = "bcrypt"
	require.Equal(t, expected, mustQuery(t)(query.selectQueryByUsername()))
}

func TestAuthHandler_basicPasswordCheck_bcryptLegacyMD5Stored(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)
	cfg := testAuthConfig()
	cfg.Encrypt = "bcrypt"
	h := NewAuthHandler(executor, pgDialect{}, cfg)

	executor.EXPECT().
		Query(mustQuery(t)(h.selectQueryByUsername()), "carol").
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		row := dest.(*loginRow)
		*row = loginRow{ID: 3, Username: "carol", Password: md5Hex("pw")}
		return 1, nil
	})

	user, err := h.basicPasswordCheck("carol", "pw")
	require.NoError(t, err)
	require.Equal(t, "carol", user.Username)
}

func TestAuthHandler_basicPasswordCheck_bcryptLegacySHA1Stored(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)
	cfg := testAuthConfig()
	cfg.Encrypt = "bcrypt"
	h := NewAuthHandler(executor, pgDialect{}, cfg)
	sha1Hex := fmt.Sprintf("%x", sha1.Sum([]byte("pw")))

	executor.EXPECT().
		Query(mustQuery(t)(h.selectQueryByUsername()), "carol").
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		row := dest.(*loginRow)
		*row = loginRow{ID: 3, Username: "carol", Password: sha1Hex}
		return 1, nil
	})

	user, err := h.basicPasswordCheck("carol", "pw")
	require.NoError(t, err)
	require.Equal(t, "carol", user.Username)
}

func TestAuthHandler_basicPasswordCheck_bcrypt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	hash, err := HashPassword("pw")
	require.NoError(t, err)

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)
	cfg := testAuthConfig()
	cfg.Encrypt = "bcrypt"
	h := NewAuthHandler(executor, pgDialect{}, cfg)

	executor.EXPECT().
		Query(mustQuery(t)(h.selectQueryByUsername()), "carol").
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		row := dest.(*loginRow)
		*row = loginRow{ID: 3, Username: "carol", Password: hash}
		return 1, nil
	})

	user, err := h.basicPasswordCheck("carol", "pw")
	require.NoError(t, err)
	require.Equal(t, "carol", user.Username)
}

func TestAuthHandler_basicPasswordCheck_bcryptWrongPassword(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	hash, err := HashPassword("pw")
	require.NoError(t, err)

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)
	cfg := testAuthConfig()
	cfg.Encrypt = "bcrypt"
	h := NewAuthHandler(executor, pgDialect{}, cfg)

	executor.EXPECT().
		Query(mustQuery(t)(h.selectQueryByUsername()), "carol").
		Return(sc)
	sc.EXPECT().Err().Return(nil)
	sc.EXPECT().Scan(gomock.Any()).DoAndReturn(func(dest interface{}) (int, error) {
		row := dest.(*loginRow)
		*row = loginRow{ID: 3, Username: "carol", Password: hash}
		return 1, nil
	})

	_, err = h.basicPasswordCheck("carol", "wrong")
	require.ErrorIs(t, err, ErrUserNotFound)
}

func TestAuthHandler_basicPasswordCheck_unknownAlgorithm(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, pgDialect{}, AuthConfig{Encrypt: "PLAINTEXT"})
	_, err := h.basicPasswordCheck("carol", "pw")
	require.ErrorIs(t, err, ErrUnknownEncryptAlgorithm)
}

func TestAuthHandler_basicPasswordCheck(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	sc := mockgen.NewMockScanner(ctrl)
	h := NewAuthHandler(executor, pgDialect{}, testAuthConfig())

	executor.EXPECT().
		Query(mustQuery(t)(h.selectQuery()), "carol", md5Hex("pw")).
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
	t.Parallel()

	user := auth.User{ID: 7, Username: "legacy"}
	token, err := Token(user, testAuthJWTKey)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	parsed, err := jwt.ParseSigned(token, []jose.SignatureAlgorithm{jose.HS256})
	require.NoError(t, err)
	var claims auth.Claims
	require.NoError(t, parsed.Claims([]byte(testAuthJWTKey), &claims))
	require.Equal(t, user.Username, claims.UserInfo.Username)
}

func TestAuthHandler_verifyStoredPassword_BcryptSuccessAndFailure(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, pgDialect{}, testAuthConfig())
	h.cfg.Encrypt = "bcrypt"

	hash, err := HashPassword("secret")
	require.NoError(t, err)

	require.NoError(t, h.verifyStoredPassword("secret", hash))
	require.ErrorIs(t, h.verifyStoredPassword("wrong", hash), ErrUserNotFound)
}

func TestAuthHandler_verifyStoredPassword_LegacyDigestAndInvalid(t *testing.T) {
	t.Parallel()

	h := NewAuthHandler(nil, pgDialect{}, testAuthConfig())

	md5Digest, err := h.legacyDigestForAlgorithm("secret", "md5")
	require.NoError(t, err)
	require.NoError(t, h.verifyStoredPassword("secret", md5Digest))
	require.ErrorIs(t, h.verifyStoredPassword("wrong", md5Digest), ErrUserNotFound)

	sha1Digest, err := h.legacyDigestForAlgorithm("secret", "sha1")
	require.NoError(t, err)
	require.NoError(t, h.verifyStoredPassword("secret", sha1Digest))
	require.ErrorIs(t, h.verifyStoredPassword("wrong", sha1Digest), ErrUserNotFound)

	require.Equal(t, "MD5", storedLegacyDigestAlgorithm(md5Digest))
	require.Equal(t, "SHA1", storedLegacyDigestAlgorithm(sha1Digest))
	require.Equal(t, "", storedLegacyDigestAlgorithm("not-hex"))
	require.True(t, isHexDigest(md5Digest))
	require.False(t, isHexDigest("xyz"))
}

func TestAuthLookupQueryMySQLDialect(t *testing.T) {
	t.Parallel()

	cfg := testAuthConfig()
	cfg.Schema = "shop"
	h := NewAuthHandler(nil, mysqlDialect{}, cfg)

	require.Equal(t, "SELECT * FROM `shop`.`prest_users` WHERE `username`=? LIMIT 1", mustQuery(t)(h.selectQueryByUsername()))
	require.Equal(t, "SELECT * FROM `shop`.`prest_users` WHERE `username`=? AND `password`=? LIMIT 1", mustQuery(t)(h.selectQuery()))
}

func TestAuthLookupQueryRejectsInvalidIdentifiers(t *testing.T) {
	t.Parallel()

	for _, mutate := range []func(*AuthConfig){
		func(c *AuthConfig) { c.Table = "users;drop" },
		func(c *AuthConfig) { c.Username = "user name" },
		func(c *AuthConfig) { c.Password = `pass"word` },
	} {
		cfg := testAuthConfig()
		mutate(&cfg)
		_, err := NewAuthHandler(nil, pgDialect{}, cfg).selectQuery()
		require.Error(t, err)
	}
	_, err := NewAuthHandler(nil, nil, testAuthConfig()).selectQuery()
	require.ErrorIs(t, err, ErrAuthDialectMissing)
}

func TestAuthHandler_Login_InvalidTableSkipsQuery(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// No EXPECT: the executor must not be called when the table name is unsafe.
	executor := mockgen.NewMockQueryExecutor(ctrl)
	cfg := testAuthConfig()
	cfg.Table = "users;drop"
	h := NewAuthHandler(executor, pgDialect{}, cfg)
	req := httptest.NewRequest(http.MethodPost, "/auth", bytes.NewBufferString(`{"username":"a","password":"b"}`))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
