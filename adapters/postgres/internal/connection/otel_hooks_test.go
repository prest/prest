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

// A connection opened after SetDriverName/SetAfterConnect uses the overridden
// driver name and invokes the post-connect hook. Mutates package globals, so it
// must not run in parallel; globals are restored on cleanup.
func TestSetDriverName_and_AfterConnect(t *testing.T) {
	origConnect := dbConnect
	origDriver := driverName
	origAfter := afterConnect
	t.Cleanup(func() {
		dbConnect = origConnect
		driverName = origDriver
		afterConnect = origAfter
	})

	var gotDriver string
	dbConnect = func(name, _ string) (*sqlx.DB, error) {
		gotDriver = name
		mockDB, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	}

	var hookCalls int32
	SetDriverName("postgres-otel")
	SetAfterConnect(func(*sql.DB) { atomic.AddInt32(&hookCalls, 1) })

	m := NewManager(&config.Prest{PGDatabase: "testdb", PGSSLMode: "disable"})
	db, err := m.AddDatabaseToPool("testdb")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.Equal(t, "postgres-otel", gotDriver)
	require.Equal(t, int32(1), atomic.LoadInt32(&hookCalls))
}

// SetDriverName ignores an empty name to avoid breaking the default driver.
func TestSetDriverName_emptyIgnored(t *testing.T) {
	origDriver := driverName
	t.Cleanup(func() { driverName = origDriver })

	SetDriverName("custom")
	SetDriverName("")
	require.Equal(t, "custom", driverName)
}
