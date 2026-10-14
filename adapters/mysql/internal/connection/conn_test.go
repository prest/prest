package connection

import (
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func testCfg() *config.Prest {
	return &config.Prest{
		PGHost:        "db.internal",
		PGPort:        3306,
		PGUser:        "app",
		PGPass:        "pw",
		PGDatabase:    "shop",
		PGSSLMode:     "disable",
		PGMaxIdleConn: 1,
		PGMaxOpenConn: 2,
	}
}

func TestManagerOpenOwnsAndClose(t *testing.T) {
	raw, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })

	restore := SetDBConnectForTest(func(driver, dsn string) (*sqlx.DB, error) {
		require.Equal(t, "mysql", driver)
		require.Contains(t, dsn, "tcp(db.internal:3306)/shop")
		require.Contains(t, dsn, "multiStatements=false")
		require.Contains(t, dsn, "interpolateParams=true")
		return sqlx.NewDb(raw, "sqlmock"), nil
	})
	t.Cleanup(restore)

	m := NewManager(testCfg())
	require.Equal(t, "", m.GetDatabase())
	m.SetDatabase("shop")
	require.Equal(t, "shop", m.GetDatabase())
	require.True(t, m.Owns(""))
	require.True(t, m.Owns("shop"))
	require.False(t, m.Owns("other"))

	_, err = m.GetOwned("other")
	require.Error(t, err)

	db, err := m.Get()
	require.NoError(t, err)
	require.NotNil(t, db)
	again, err := m.Get()
	require.NoError(t, err)
	require.Equal(t, db, again)
	owned, err := m.GetOwned("shop")
	require.NoError(t, err)
	require.Equal(t, db, owned)

	dsn, err := m.DSN()
	require.NoError(t, err)
	require.NotContains(t, RedactedDSN(dsn), "pw")

	m.CloseAll()
	require.Empty(t, m.getPool().DB)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestOwns_AllowListed: without a registry pg.allowed_databases are owned;
// with a registry only the alias and its physical database are.
func TestOwns_AllowListed(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.AllowedDatabases = []string{"other"}
	require.True(t, NewManager(cfg).Owns("other"))
	require.False(t, NewManager(cfg).Owns("mysql"))

	reg := testCfg()
	reg.AllowedDatabases = []string{"other"}
	reg.Databases = []config.DatabaseConf{{Alias: "store"}}
	m := NewManager(reg)
	m.SetDatabase("store")
	require.False(t, m.Owns("other"))
	require.True(t, m.Owns("store"))
	require.True(t, m.Owns("shop"))
	require.False(t, NewManager(nil).Owns("other"))
}

// TestGetOwned_OpensAllowListedLazily: an allow-listed name dials its own
// database on first use, reuses that pool afterwards, and applies pool limits.
func TestGetOwned_OpensAllowListedLazily(t *testing.T) {
	raw, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	var dsns []string
	restore := SetDBConnectForTest(func(_, dsn string) (*sqlx.DB, error) {
		dsns = append(dsns, dsn)
		return sqlx.NewDb(raw, "sqlmock"), nil
	})
	t.Cleanup(restore)

	cfg := testCfg()
	cfg.AllowedDatabases = []string{"other"}
	m := NewManager(cfg)
	db, err := m.GetOwned("other")
	require.NoError(t, err)
	require.Len(t, dsns, 1)
	require.Contains(t, dsns[0], "tcp(db.internal:3306)/other?")
	require.Equal(t, 2, db.Stats().MaxOpenConnections)

	again, err := m.GetOwned("other")
	require.NoError(t, err)
	require.Same(t, db, again)
	require.Len(t, dsns, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGetOwned_OpensRegistryAliasLazily: a lazy registry adapter that never
// ran Connect opens its pool on the first request (regression for problem 3).
func TestGetOwned_OpensRegistryAliasLazily(t *testing.T) {
	raw, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	calls := 0
	restore := SetDBConnectForTest(func(_, dsn string) (*sqlx.DB, error) {
		calls++
		require.Contains(t, dsn, "/shop?")
		return sqlx.NewDb(raw, "sqlmock"), nil
	})
	t.Cleanup(restore)

	cfg := testCfg()
	cfg.Databases = []config.DatabaseConf{{Alias: "store"}}
	m := NewManager(cfg)
	m.SetDatabase("store")
	db, err := m.GetOwned("store")
	require.NoError(t, err)
	require.NotNil(t, db)
	viaGet, err := m.Get()
	require.NoError(t, err)
	require.Same(t, db, viaGet)
	require.Equal(t, 1, calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGetOwned_ConnectErrorWrapped keeps the dial error reachable with
// errors.As so callers can classify it, without leaking the password.
func TestGetOwned_ConnectErrorWrapped(t *testing.T) {
	restore := SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) { return nil, errOpen })
	t.Cleanup(restore)
	cfg := testCfg()
	cfg.AllowedDatabases = []string{"other"}
	_, err := NewManager(cfg).GetOwned("other")
	require.ErrorIs(t, err, errOpen)
	require.NotContains(t, err.Error(), "pw")
}

// TestDSNFor: the alias, pg.database and "" use DSN(); an allow-listed name
// swaps only the database.
func TestDSNFor(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.AllowedDatabases = []string{"other"}
	m := NewManager(cfg)
	m.SetDatabase("main")
	base, err := m.DSN()
	require.NoError(t, err)
	for _, name := range []string{"", "main", "shop"} {
		got, err := m.dsnFor(name)
		require.NoError(t, err)
		require.Equal(t, base, got, name)
	}
	other, err := m.dsnFor("other")
	require.NoError(t, err)
	require.Equal(t, strings.Replace(base, "/shop?", "/other?", 1), other)

	_, err = NewManager(nil).dsnFor("other")
	require.Error(t, err)
}

func TestManagerDSNPrepare(t *testing.T) {
	t.Parallel()
	cfg := testCfg()
	cfg.MySQLPrepare = true
	dsn, err := NewManager(cfg).DSN()
	require.NoError(t, err)
	require.Contains(t, dsn, "interpolateParams=false")
	require.Contains(t, dsn, "multiStatements=false")
}

func TestManagerDSNErrors(t *testing.T) {
	t.Parallel()
	_, err := NewManager(nil).DSN()
	require.Error(t, err)
	_, err = NewManager(nil).Get()
	require.Error(t, err)

	cfg := testCfg()
	cfg.PGSSLCert = "/tmp/cert.pem"
	_, err = NewManager(cfg).Get()
	require.Error(t, err)

	cfg = testCfg()
	cfg.PGUser = ""
	_, err = NewManager(cfg).GetOwned("")
	require.Error(t, err)
}

func TestManagerConnectFailure(t *testing.T) {
	restore := SetDBConnectForTest(func(_, _ string) (*sqlx.DB, error) {
		return nil, errOpen
	})
	t.Cleanup(restore)
	_, err := NewManager(testCfg()).Get()
	require.Error(t, err)
	require.Contains(t, err.Error(), "mysql://")
	require.NotContains(t, err.Error(), "pw")
}

var errOpen = errString("dial failed")

type errString string

func (e errString) Error() string { return string(e) }
