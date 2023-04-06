package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers/auth"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

func TestJWTClaimsOk(t *testing.T) {
	app = nil
	MiddlewareStack = nil
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS512")
	config.Load()
	nd := appTestWithJwt()
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
	app = nil
	t.Setenv("PREST_JWT_DEFAULT", "true")
	t.Setenv("PREST_DEBUG", "false")
	t.Setenv("PREST_JWT_KEY", "s3cr3t")
	t.Setenv("PREST_JWT_ALGO", "HS256")
	config.Load()
	nd := appTestWithJwt()
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
