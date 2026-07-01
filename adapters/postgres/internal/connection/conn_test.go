package connection

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/config"
	"github.com/stretchr/testify/require"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(&config.Prest{
		PGDatabase: "testdb",
		PGHost:     "localhost",
		PGPort:     5432,
		PGUser:     "u",
		PGPass:     "secret",
		PGSSLMode:  "disable",
	})
}

func TestGetFromPool_returnsInjectedDB(t *testing.T) {
	m := testManager(t)
	uri := m.GetURI("testdb")

	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	m.InjectDBForTest(uri, sqlxDB)

	got, err := m.GetFromPool("testdb")
	require.NoError(t, err)
	require.Same(t, sqlxDB, got)
}

func TestAddDatabaseToPool_returnsExistingWithoutConnect(t *testing.T) {
	m := testManager(t)
	uri := m.GetURI("testdb")

	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	m.InjectDBForTest(uri, sqlxDB)

	got, err := m.AddDatabaseToPool("testdb")
	require.NoError(t, err)
	require.Same(t, sqlxDB, got)
}

func TestAddDatabaseToPool_singleflightDedup(t *testing.T) {
	m := testManager(t)
	uri := m.GetURI("otherdb")

	var connectCalls int32
	origConnect := dbConnect
	dbConnect = func(driverName, dataSourceName string) (*sqlx.DB, error) {
		require.Equal(t, "postgres", driverName)
		require.Equal(t, uri, dataSourceName)
		atomic.AddInt32(&connectCalls, 1)
		time.Sleep(25 * time.Millisecond)
		mockDB, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	}
	t.Cleanup(func() { dbConnect = origConnect })

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	errs := make([]error, workers)
	dbs := make([]*sqlx.DB, workers)

	for i := range workers {
		go func(idx int) {
			defer wg.Done()
			dbs[idx], errs[idx] = m.AddDatabaseToPool("otherdb")
		}(i)
	}
	wg.Wait()

	require.Equal(t, int32(1), connectCalls)
	for i := range workers {
		require.NoError(t, errs[i])
		require.NotNil(t, dbs[i])
	}
	for i := 1; i < workers; i++ {
		require.Same(t, dbs[0], dbs[i])
	}
}

func TestGetDatabaseFromPool_concurrentReads(t *testing.T) {
	m := testManager(t)
	uri := m.GetURI("testdb")

	mockDB, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = mockDB.Close() })

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	m.InjectDBForTest(uri, sqlxDB)

	const readers = 32
	var wg sync.WaitGroup
	wg.Add(readers)
	for range readers {
		go func() {
			defer wg.Done()
			got := m.getDatabaseFromPool("testdb")
			require.Same(t, sqlxDB, got)
		}()
	}
	wg.Wait()
}
