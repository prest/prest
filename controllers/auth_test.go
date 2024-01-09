package controllers

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/prest/prest/adapters/mockgen"
	"github.com/prest/prest/config"
	"github.com/prest/prest/controllers/auth"
)

var (
	defaultConfig = &config.Prest{
		AuthEnabled:  true,
		AuthEncrypt:  "MD5",
		AuthSchema:   "public",
		AuthTable:    "prest_users",
		AuthUsername: "username",
		AuthPassword: "password",
		Debug:        true,
	}

	authUser = auth.User{
		ID:       1,
		Name:     "prest-user",
		Username: "arxdsilva",
	}
)

func Test_basicPasswordCheck_ok(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)

	ctrl2 := gomock.NewController(t)
	adapter2 := mockgen.NewMockScanner(ctrl2)

	adapter.EXPECT().QueryCtx(gomock.Any(),
		"SELECT * FROM public.prest_users WHERE username=$1 AND password=$2 LIMIT 1",
		"test@postgres.rest", "e10adc3949ba59abbe56e057f20f883e").
		Return(adapter2)

	adapter2.EXPECT().Err().Return(nil)
	adapter2.EXPECT().Scan(&auth.User{}).Return(1, nil)

	cfg := &Config{
		server:  defaultConfig,
		adapter: adapter,
	}

	_, err := cfg.basicPasswordCheck(context.Background(), "test@postgres.rest", "123456")
	require.NoError(t, err)
}

func Test_basicPasswordCheck_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := mockgen.NewMockAdapter(ctrl)

	ctrl2 := gomock.NewController(t)
	adapter2 := mockgen.NewMockScanner(ctrl2)

	adapter.EXPECT().QueryCtx(gomock.Any(),
		"SELECT * FROM public.prest_users WHERE username=$1 AND password=$2 LIMIT 1",
		"test@postgres.rest", "e10adc3949ba59abbe56e057f20f883e").
		Return(adapter2)

	adapter2.EXPECT().Err().Return(nil)
	adapter2.EXPECT().Scan(&auth.User{}).Return(0, nil)

	cfg := &Config{
		server:  defaultConfig,
		adapter: adapter,
	}

	_, err := cfg.basicPasswordCheck(context.Background(), "test@postgres.rest", "123456")
	require.Error(t, err)
	require.Equal(t, unf, err.Error())
}

func Test_getSelectQuery(t *testing.T) {
	cfg := &Config{server: defaultConfig}
	expected := "SELECT * FROM public.prest_users WHERE username=$1 AND password=$2 LIMIT 1"
	query := cfg.getSelectQuery()

	require.Equal(t, expected, query)
}

func Test_encrypt(t *testing.T) {
	cfg := &Config{
		server:  &config.Prest{AuthEncrypt: "MD5"},
		adapter: nil,
	}

	pwd := "123456"
	enc := encrypt(cfg.server.AuthEncrypt, pwd)

	md5Enc := fmt.Sprintf("%x", md5.Sum([]byte(pwd)))
	require.Equal(t, enc, md5Enc)

	cfg.server.AuthEncrypt = "SHA1"

	enc = encrypt(cfg.server.AuthEncrypt, pwd)

	sha1Enc := fmt.Sprintf("%x", sha1.Sum([]byte(pwd)))
	require.Equal(t, enc, sha1Enc)
}

// func Test_AuthController(t *testing.T) {
// 	var testCases = []struct {
// 		description string

// 		request *http.Request
// 		config  *Config

// 		wantStatus int

// 		wantPassCheck bool
// 		wantPassResp auth.User
// 	}{
// 		{"/auth request GET method", "/auth", "GET", http.StatusMethodNotAllowed, false, ""},
// 		{"/auth request POST method basic auth", "/auth", "POST", http.StatusBadRequest, false, "basic"},
// 		{"/auth request POST method no auth provided", "/auth", "POST", http.StatusUnauthorized, true, ""},
// 	}

// 	for _, tc := range testCases {
// 		t.Log(tc.description)

// 		ctrl := gomock.NewController(t)
// 		adapter := mockgen.NewMockAdapter(ctrl)

// 		if tc.wantPassCheck {
// 			ctrl2 := gomock.NewController(t)
// 			adapter2 := mockgen.NewMockScanner(ctrl2)

// 			adapter.EXPECT().QueryCtx(gomock.Any(), "SELECT * FROM . WHERE =$1 AND =$2 LIMIT 1",
// 				gomock.Any(), gomock.Any()).Return(adapter2)

// 			adapter2.EXPECT().Err().Return(nil)
// 			adapter2.EXPECT().Scan(&auth.User{}).Return(0, nil)
// 		}

// 		h := Config{
// 			server: &config.Prest{
// 				Debug:       true,
// 				AuthEnabled: true,
// 				AuthType:    tc.authType,
// 			},
// 			adapter: adapter,
// 		}

// 		server := httptest.NewServer(initAuthRoutes(true, h))

// 		testutils.DoRequest(t, server.URL+tc.url, nil, tc.method, tc.status, "AuthEnable")

// 		server.Close()
// 	}
// }

func Test_Token(t *testing.T) {
	u := auth.User{
		ID:       1,
		Name:     "prest-user",
		Username: "arxdsilva",
	}
	key := "secret-key"

	t.Run("Token generation", func(t *testing.T) {
		token, err := Token(u, key)
		require.NoError(t, err)
		require.NotEmpty(t, token)
	})

	t.Run("Token verification", func(t *testing.T) {
		token, err := Token(u, key)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		// TODO: Implement token verification test
	})
}
