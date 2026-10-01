package connection

import (
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
	_, err = m.GetOwned("shop")
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
	_, err = m.GetOwned("shop")
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
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
