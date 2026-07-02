package app_test

import (
	"testing"

	"github.com/prest/prest/v2/adapters/mock"
	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func TestPostgresDB_ErrAdapterNotPostgres(t *testing.T) {
	cfg := &config.Prest{
		Adapter: &mock.Mock{},
	}
	_, err := app.PostgresDB(cfg)
	require.ErrorIs(t, err, app.ErrAdapterNotPostgres)
}

func TestPostgresDB_EnsureAdapterConnects(t *testing.T) {
	cfg := &config.Prest{
		PGHost:     "invalid-host",
		PGPort:     1,
		PGUser:     "x",
		PGDatabase: "x",
		PGSSLMode:  "disable",
	}
	_, err := app.PostgresDB(cfg)
	require.Error(t, err)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) *config.Prest
		wantErr bool
	}{
		{
			name: "success with existing adapter",
			setup: func(t *testing.T) *config.Prest {
				return &config.Prest{
					Adapter:    mock.New(t),
					PGDatabase: "prest",
				}
			},
		},
		{
			name: "connect error when adapter is nil",
			setup: func(t *testing.T) *config.Prest {
				return &config.Prest{
					PGHost:     "invalid-host",
					PGPort:     1,
					PGUser:     "x",
					PGDatabase: "x",
					PGSSLMode:  "disable",
				}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setup(t)
			adapterBefore := cfg.Adapter

			got, err := app.New(cfg)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			require.NotNil(t, got.Handler)
			require.Same(t, cfg, got.Config)
			if adapterBefore != nil {
				require.Same(t, adapterBefore, cfg.Adapter)
			}
		})
	}
}
