package controllers

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/cache"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestNewDepsFromConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := mockgen.NewMockAdapter(ctrl)
	p := &config.Prest{
		Adapter:      adapter,
		SingleDB:     true,
		PGDatabase:   "prest-test",
		AuthEnabled:  true,
		AuthType:     "body",
		JWTKey:       "secret",
		AuthSchema:   "public",
		AuthTable:    "users",
		AuthUsername: "username",
		AuthPassword: "password",
		AuthEncrypt:  "MD5",
		Cache:        cache.Config{Enabled: true},
	}

	deps := NewDepsFromConfig(p)
	require.Equal(t, adapter, deps.Catalog)
	require.Equal(t, adapter, deps.DB)
	require.Equal(t, adapter, deps.Executor)
	require.True(t, deps.SingleDB)
	require.Equal(t, "prest-test", deps.PGDatabase)
	require.NotNil(t, deps.Cache)
	require.True(t, deps.Auth.Enabled)
	require.Equal(t, "body", deps.Auth.AuthType)
	require.Equal(t, "secret", deps.Auth.JWTKey)
}

func TestNewDepsFromConfig_CacheDisabled(t *testing.T) {
	t.Parallel()

	p := &config.Prest{Cache: cache.Config{Enabled: false}}
	deps := NewDepsFromConfig(p)
	require.Nil(t, deps.Cache)
}

func TestNewHandlers(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := mockgen.NewMockAdapter(ctrl)
	h := NewHandlers(Deps{
		Catalog:  adapter,
		Builder:  adapter,
		Executor: adapter,
		SQL:      adapter,
		Perms:    adapter,
		Scripts:  adapter,
		DB:       adapter,
	}, nil)

	require.NotNil(t, h.Auth)
	require.NotNil(t, h.Catalog)
	require.NotNil(t, h.MCP)
	require.NotNil(t, h.Table)
	require.NotNil(t, h.CRUD)
	require.NotNil(t, h.Script)
	require.NotNil(t, h.Health)
}

func TestNewHandlersFromConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := mockgen.NewMockAdapter(ctrl)
	h := NewHandlersFromConfig(&config.Prest{Adapter: adapter})
	require.NotNil(t, h.CRUD)
}

func TestNewHandlers_QueryRegistryRequiresAdapter(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := mockgen.NewMockAdapter(ctrl)
	cfg := &config.Prest{
		QueriesConf: config.QueriesConf{
			RegisterEnabled: true,
			Storage:         config.QueriesStorageDatabase,
		},
	}

	h := NewHandlers(Deps{
		Catalog:  adapter,
		Builder:  adapter,
		Executor: adapter,
		SQL:      adapter,
		Perms:    adapter,
		Scripts:  adapter,
		DB:       adapter,
	}, cfg)
	require.Nil(t, h.QueryRegistry)
}

func TestNewHandlers_QueryRegistryWithAdapter(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	registry := mockgen.NewMockQueryRegistry(ctrl)
	cfg := &config.Prest{
		QueriesConf: config.QueriesConf{
			RegisterEnabled: true,
			Storage:         config.QueriesStorageDatabase,
		},
	}

	h := NewHandlers(Deps{
		QueryRegistry: registry,
		DB:            mockgen.NewMockAdapter(ctrl),
	}, cfg)
	require.NotNil(t, h.QueryRegistry)
}
