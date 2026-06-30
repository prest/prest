package middlewares

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/controllers/auth"

	"github.com/gorilla/mux"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/require"
	"github.com/urfave/negroni/v3"
	jose "gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

func appTestWithJwt(t *testing.T) *negroni.Negroni {
	ResetForTest()
	t.Cleanup(ResetForTest)

	n := GetApp()
	r := mux.NewRouter()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test app"))
	}).Methods("GET")
	n.UseHandler(r)
	return n
}

func TestJWTClaimsOk(t *testing.T) {
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS512")
	config.Load()
	nd := appTestWithJwt(t)
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
			Key:       []byte(config.PrestConf.JWTKey)},
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
	nd := appTestWithJwt(t)
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
			Key:       []byte(config.PrestConf.JWTKey)},
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
	nd := appTestWithJwt(t)
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
	nd := appTestWithJwt(t)
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
	// MatchURL reads config.PrestConf.JWTWhiteList; initialize an empty
	// config so the middleware can run in isolation without depending on
	// env-driven config.Load() side effects.
	config.PrestConf = &config.Prest{}
	mw := JwtMiddleware("", "", "HS256")

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
	// MatchURL reads config.PrestConf.JWTWhiteList; initialize an empty
	// config so the middleware can run in isolation without depending on
	// env-driven config.Load() side effects.
	config.PrestConf = &config.Prest{}

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

	mw := JwtMiddleware("", string(jwksJSON), "HS256")

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
