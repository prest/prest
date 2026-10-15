package app_test

import (
	"fmt"
	"testing"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mysql"
	"github.com/prest/prest/v2/app"
	"github.com/prest/prest/v2/config"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

// TestNew_Registry_OneUnreachable_StartsAndRegistersLazy: one of two registry
// entries refuses connections. Startup succeeds, the dead alias is served by an
// unconnected postgres adapter, and it was dialled once (no Timescale→Postgres
// retry, no dial when building the lazy adapter).
func TestNew_Registry_OneUnreachable_StartsAndRegistersLazy(t *testing.T) {
	dials := stubConnectFailure(t, "down_db")

	cfg := &config.Prest{Databases: []config.DatabaseConf{baseDBConf("down"), baseDBConf("live")}}
	got, err := app.New(cfg)
	require.NoError(t, err)
	require.True(t, got.Adapters.IsRegistered("live"))
	require.True(t, got.Adapters.IsRegistered("down"))

	down, err := got.Adapters.Get("down")
	require.NoError(t, err)
	require.Equal(t, "*postgres.postgres", fmt.Sprintf("%T", down))
	require.Equal(t, 1, dials["down_db"])

	live, err := got.Adapters.Get("live")
	require.NoError(t, err)
	require.Same(t, live, cfg.Adapter, "default adapter must be the reachable alias")
}

// TestNew_Registry_MySQLUnreachable_RegistersLazyMySQL: an unreachable mysql
// entry keeps its engine, so requests reach the mysql adapter once it is up.
func TestNew_Registry_MySQLUnreachable_RegistersLazyMySQL(t *testing.T) {
	stubPostgresFallbackConnect(t)
	restore := mysql.SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, refusedErr()
	})
	t.Cleanup(restore)

	shop := baseDBConf("shop")
	shop.Engine = config.EngineMySQL
	shop.Port = 3306
	cfg := &config.Prest{Databases: []config.DatabaseConf{baseDBConf("pg"), shop}}
	got, err := app.New(cfg)
	require.NoError(t, err)

	adapter, err := got.Adapters.Get("shop")
	require.NoError(t, err)
	my, ok := adapter.(*mysql.Adapter)
	require.True(t, ok)
	require.Equal(t, "shop", my.GetDatabase())
}

// TestNew_Registry_AllUnreachable_Fails: when no entry can be reached the
// configuration is broken as a whole, so startup still fails.
func TestNew_Registry_AllUnreachable_Fails(t *testing.T) {
	stubConnectFailure(t, "a_db", "b_db")

	cfg := &config.Prest{Databases: []config.DatabaseConf{baseDBConf("a"), baseDBConf("b")}}
	got, err := app.New(cfg)
	require.Nil(t, got)
	require.ErrorIs(t, err, adapters.ErrUnavailable)
}

// TestNew_Registry_ConfigErrorStillFatal: an unknown engine is a config
// mistake, not an outage, and must not be turned into a lazy adapter.
func TestNew_Registry_ConfigErrorStillFatal(t *testing.T) {
	stubPostgresFallbackConnect(t)

	bad := baseDBConf("bad")
	bad.Engine = "oracle"
	cfg := &config.Prest{Databases: []config.DatabaseConf{baseDBConf("good"), bad}}
	_, err := app.New(cfg)
	require.Error(t, err)
	require.False(t, adapters.IsUnavailable(err))
}

// TestNew_DetectAdapter_DialError_NoPostgresRetry: single-database mode keeps
// failing fast, after a single dial instead of Timescale plus Postgres.
func TestNew_DetectAdapter_DialError_NoPostgresRetry(t *testing.T) {
	dials := stubConnectFailure(t, "prest")

	got, err := app.New(connectablePrest("prest"))
	require.Nil(t, got)
	require.True(t, adapters.IsUnavailable(err))
	require.Equal(t, 1, dials["prest"])
}

// TestNew_DefaultAdapterIsFirstConfiguredAlias: the default adapter (health,
// script permissions, requests without {database}) is the first configured
// alias on every start, not whatever map iteration returns.
func TestNew_DefaultAdapterIsFirstConfiguredAlias(t *testing.T) {
	stubPostgresFallbackConnect(t)

	for range 20 {
		cfg := &config.Prest{Databases: []config.DatabaseConf{baseDBConf("a"), baseDBConf("b"), baseDBConf("c")}}
		got, err := app.New(cfg)
		require.NoError(t, err)
		want, err := got.Adapters.Get("a")
		require.NoError(t, err)
		require.Same(t, want, cfg.Adapter)
	}
}
