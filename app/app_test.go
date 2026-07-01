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
