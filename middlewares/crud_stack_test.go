package middlewares

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/cache"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestNewCRUDStack(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter := mockgen.NewMockAdapter(ctrl)
	cfg := &config.Prest{
		Adapter: adapter,
		JWTAlgo: "HS256",
		Cache:   cache.Config{Enabled: false},
	}
	withPrestConf(t, cfg)

	stack := NewCRUDStack(cfg)
	require.Len(t, stack.Handlers(), 5)
}

func TestNewCRUDStackWithPerms(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	cfg := &config.Prest{
		JWTAlgo: "HS256",
		Cache:   cache.Config{Enabled: false},
	}
	withPrestConf(t, cfg)

	stack := NewCRUDStackWithPerms(cfg, perms)
	require.Len(t, stack.Handlers(), 5)
}
