package connection

import (
	"database/sql"
	"sync/atomic"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

// A Manager built with WithDriverName/WithAfterConnect opens connections using
// the injected driver name and invokes the post-connect hook. Uses the dbConnect
// test seam; restored on cleanup.
func TestManager_WithDriverNameAndAfterConnect(t *testing.T) {
	var gotDriver string
	restore := SetDBConnectForTest(func(name, _ string) (*sqlx.DB, error) {
		gotDriver = name
		mockDB, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	})
	t.Cleanup(restore)

	var hookCalls int32
	m := NewManager(&config.Prest{PGDatabase: "testdb", PGSSLMode: "disable"},
		WithDriverName("postgres-otel"),
		WithAfterConnect(func(*sql.DB) { atomic.AddInt32(&hookCalls, 1) }),
	)

	db, err := m.AddDatabaseToPool("testdb")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.Equal(t, "postgres-otel", gotDriver)
	require.Equal(t, int32(1), atomic.LoadInt32(&hookCalls))
}

// Without options, the Manager uses the default lib/pq "postgres" driver and no
// post-connect hook.
func TestManager_DefaultDriverName(t *testing.T) {
	var gotDriver string
	restore := SetDBConnectForTest(func(name, _ string) (*sqlx.DB, error) {
		gotDriver = name
		mockDB, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	})
	t.Cleanup(restore)

	m := NewManager(&config.Prest{PGDatabase: "testdb", PGSSLMode: "disable"})
	db, err := m.AddDatabaseToPool("testdb")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.Equal(t, defaultDriverName, gotDriver)
}

// WithDriverName ignores an empty name to preserve the default driver.
func TestWithDriverName_emptyIgnored(t *testing.T) {
	m := NewManager(&config.Prest{}, WithDriverName(""))
	require.Equal(t, defaultDriverName, m.driverName)
}
