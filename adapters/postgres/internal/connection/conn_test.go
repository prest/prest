package connection

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/adapters"
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

func TestManager_poolLimitsFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cfg         *config.Prest
		dbName      string
		wantMaxIdle int
		wantMaxOpen int
	}{
		{
			name: "legacy mode uses global limits",
			cfg: &config.Prest{
				PGMaxIdleConn: 2,
				PGMaxOpenConn: 10,
			},
			dbName:      "legacydb",
			wantMaxIdle: 2,
			wantMaxOpen: 10,
		},
		{
			name: "registry alias uses per-database limits",
			cfg: &config.Prest{
				PGMaxIdleConn: 2,
				PGMaxOpenConn: 10,
				Databases: []config.DatabaseConf{
					{Alias: "tenant-a", MaxIdleConn: 5, MaxOpenConn: 25},
				},
			},
			dbName:      "tenant-a",
			wantMaxIdle: 5,
			wantMaxOpen: 25,
		},
		{
			name: "unknown alias falls back to global limits",
			cfg: &config.Prest{
				PGMaxIdleConn: 2,
				PGMaxOpenConn: 10,
				Databases: []config.DatabaseConf{
					{Alias: "tenant-a", MaxIdleConn: 5, MaxOpenConn: 25},
				},
			},
			dbName:      "otherdb",
			wantMaxIdle: 2,
			wantMaxOpen: 10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.cfg)
			gotIdle, gotOpen := m.poolLimitsFor(tt.dbName)
			require.Equal(t, tt.wantMaxIdle, gotIdle)
			require.Equal(t, tt.wantMaxOpen, gotOpen)
		})
	}
}

func TestAddDatabaseToPool_appliesPoolLimits(t *testing.T) {
	m := NewManager(&config.Prest{
		PGMaxIdleConn: 2,
		PGMaxOpenConn: 10,
		Databases: []config.DatabaseConf{
			{Alias: "tenant-a", MaxIdleConn: 5, MaxOpenConn: 25},
		},
	})

	origConnect := dbConnect
	dbConnect = func(driverName, dataSourceName string) (*sqlx.DB, error) {
		mockDB, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	}
	t.Cleanup(func() { dbConnect = origConnect })

	db, err := m.AddDatabaseToPool("tenant-a")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.Equal(t, 25, db.Stats().MaxOpenConnections)
}

// Once maxPools distinct pools exist, a new name is refused with
// adapters.ErrUnavailable (HTTP 503) and no connection is attempted. Names
// already pooled keep working.
func TestAddDatabaseToPool_CapsDistinctPools(t *testing.T) {
	m := testManager(t)

	var connectCalls int32
	origConnect := dbConnect
	dbConnect = func(driverName, dataSourceName string) (*sqlx.DB, error) {
		atomic.AddInt32(&connectCalls, 1)
		mockDB, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	}
	t.Cleanup(func() { dbConnect = origConnect })
	t.Cleanup(func() {
		for _, db := range m.GetPool().DB {
			_ = db.Close()
		}
	})

	for i := range maxPools {
		_, err := m.AddDatabaseToPool(fmt.Sprintf("db%d", i))
		require.NoError(t, err)
	}
	require.Equal(t, int32(maxPools), connectCalls)

	_, err := m.AddDatabaseToPool("one-too-many")
	require.Error(t, err)
	require.ErrorIs(t, err, adapters.ErrUnavailable)
	require.True(t, adapters.IsUnavailable(err))
	require.Equal(t, int32(maxPools), connectCalls)
	require.Len(t, m.GetPool().DB, maxPools)

	got, err := m.AddDatabaseToPool("db0")
	require.NoError(t, err)
	require.NotNil(t, got)
}

// The cap is rechecked after connecting: if other names filled the last slot
// meanwhile, the fresh connection is closed and ErrUnavailable returned.
func TestAddDatabaseToPool_CapRecheckedAfterConnect(t *testing.T) {
	m := testManager(t)

	origConnect := dbConnect
	dbConnect = func(driverName, dataSourceName string) (*sqlx.DB, error) {
		for i := range maxPools {
			mockDB, _, err := sqlmock.New()
			if err != nil {
				return nil, err
			}
			m.InjectDBForTest(fmt.Sprintf("racer-%d", i), sqlx.NewDb(mockDB, "sqlmock"))
		}
		mockDB, mock, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		mock.ExpectClose()
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	}
	t.Cleanup(func() { dbConnect = origConnect })
	t.Cleanup(func() {
		for _, db := range m.GetPool().DB {
			_ = db.Close()
		}
	})

	_, err := m.AddDatabaseToPool("late")
	require.ErrorIs(t, err, adapters.ErrUnavailable)
	require.Len(t, m.GetPool().DB, maxPools)
}

func TestGetFromPool_returnsInjectedDB(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	var capturedDriver, capturedDSN string
	origConnect := dbConnect
	dbConnect = func(driverName, dataSourceName string) (*sqlx.DB, error) {
		atomic.AddInt32(&connectCalls, 1)
		capturedDriver = driverName
		capturedDSN = dataSourceName
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

	require.Equal(t, "postgres", capturedDriver)
	require.Equal(t, uri, capturedDSN)
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
	t.Parallel()

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

func TestBuildURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		conf     config.DatabaseConf
		defaults *config.Prest
		want     string
	}{
		{
			name: "URL field takes precedence",
			conf: config.DatabaseConf{
				URL:      "postgres://user:pass@localhost/db",
				Host:     "ignored",
				Port:     9999,
				User:     "ignored",
				Database: "ignored",
			},
			defaults: &config.Prest{
				PGHost: "default-host",
				PGPort: 5432,
			},
			want: "postgres://user:pass@localhost/db",
		},
		{
			name: "all config fields provided, no defaults needed",
			conf: config.DatabaseConf{
				Host:     "conf-host",
				Port:     5433,
				User:     "conf-user",
				Pass:     "conf-pass",
				Database: "conf-db",
				SSL: config.DatabaseSSLConf{
					Mode: "require",
				},
			},
			defaults: &config.Prest{
				PGHost:        "default-host",
				PGPort:        5432,
				PGUser:        "default-user",
				PGPass:        "default-pass",
				PGDatabase:    "default-db",
				PGSSLMode:     "disable",
				PGConnTimeout: 30,
			},
			want: "user=conf-user dbname=conf-db host=conf-host port=5433 sslmode=require connect_timeout=30 password=conf-pass",
		},
		{
			name: "config fields empty, uses all defaults",
			conf: config.DatabaseConf{},
			defaults: &config.Prest{
				PGHost:        "localhost",
				PGPort:        5432,
				PGUser:        "postgres",
				PGPass:        "secret",
				PGDatabase:    "mydb",
				PGSSLMode:     "disable",
				PGConnTimeout: 10,
			},
			want: "user=postgres dbname=mydb host=localhost port=5432 sslmode=disable connect_timeout=10 password=secret",
		},
		{
			name: "config database empty, uses default",
			conf: config.DatabaseConf{
				Host:     "conf-host",
				Port:     5432,
				User:     "conf-user",
				Database: "",
			},
			defaults: &config.Prest{
				PGDatabase:    "default-db",
				PGSSLMode:     "disable",
				PGConnTimeout: 10,
			},
			want: "user=conf-user dbname=default-db host=conf-host port=5432 sslmode=disable connect_timeout=10",
		},
		{
			name: "config port zero, uses default",
			conf: config.DatabaseConf{
				Host:     "conf-host",
				Port:     0,
				User:     "conf-user",
				Database: "conf-db",
			},
			defaults: &config.Prest{
				PGPort:        5432,
				PGSSLMode:     "disable",
				PGConnTimeout: 10,
			},
			want: "user=conf-user dbname=conf-db host=conf-host port=5432 sslmode=disable connect_timeout=10",
		},
		{
			name: "config password empty, uses default",
			conf: config.DatabaseConf{
				Host:     "host",
				Port:     5432,
				User:     "user",
				Pass:     "",
				Database: "db",
			},
			defaults: &config.Prest{
				PGPass:        "default-pass",
				PGSSLMode:     "disable",
				PGConnTimeout: 10,
			},
			want: "user=user dbname=db host=host port=5432 sslmode=disable connect_timeout=10 password=default-pass",
		},
		{
			name: "with SSL certificate and key",
			conf: config.DatabaseConf{
				Host:     "host",
				Port:     5432,
				User:     "user",
				Database: "db",
				SSL: config.DatabaseSSLConf{
					Mode:     "require",
					Cert:     "/path/to/cert",
					Key:      "/path/to/key",
					RootCert: "/path/to/root",
				},
			},
			defaults: &config.Prest{
				PGSSLMode:     "disable",
				PGConnTimeout: 10,
			},
			want: "user=user dbname=db host=host port=5432 sslmode=require connect_timeout=10 sslcert=/path/to/cert sslkey=/path/to/key sslrootcert=/path/to/root",
		},
		{
			name: "config SSL mode empty, uses default",
			conf: config.DatabaseConf{
				Host:     "host",
				Port:     5432,
				User:     "user",
				Database: "db",
				SSL: config.DatabaseSSLConf{
					Mode: "",
				},
			},
			defaults: &config.Prest{
				PGSSLMode:     "require",
				PGConnTimeout: 10,
			},
			want: "user=user dbname=db host=host port=5432 sslmode=require connect_timeout=10",
		},
		{
			name: "config host empty, uses default",
			conf: config.DatabaseConf{
				Host:     "",
				Port:     5432,
				User:     "user",
				Database: "db",
			},
			defaults: &config.Prest{
				PGHost:        "default-host",
				PGSSLMode:     "disable",
				PGConnTimeout: 10,
			},
			want: "user=user dbname=db host=default-host port=5432 sslmode=disable connect_timeout=10",
		},
		{
			name: "config user empty, uses default",
			conf: config.DatabaseConf{
				Host:     "host",
				Port:     5432,
				User:     "",
				Database: "db",
			},
			defaults: &config.Prest{
				PGUser:        "default-user",
				PGSSLMode:     "disable",
				PGConnTimeout: 10,
			},
			want: "user=default-user dbname=db host=host port=5432 sslmode=disable connect_timeout=10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildURI(tt.conf, tt.defaults)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestManager_SetDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string // description of this test case
		dbName       string // database name to set
		expectedName string // expected database name after set
	}{
		{
			name:         "set single database name",
			dbName:       "mydb",
			expectedName: "mydb",
		},
		{
			name:         "set empty database name",
			dbName:       "",
			expectedName: "",
		},
		{
			name:         "set database with alias",
			dbName:       "prod-db",
			expectedName: "prod-db",
		},
		{
			name:         "overwrite existing database",
			dbName:       "newdb",
			expectedName: "newdb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := testManager(t)
			m.SetDatabase(tt.dbName)
			got := m.GetDatabase()
			require.Equal(t, tt.expectedName, got)
		})
	}
}

func TestManager_SetDatabase_concurrent(t *testing.T) {
	t.Parallel()

	m := testManager(t)

	const writers = 10
	const readsPerWriter = 5

	var wg sync.WaitGroup
	wg.Add(writers)

	for i := range writers {
		go func(idx int) {
			defer wg.Done()
			for j := range readsPerWriter {
				dbName := fmt.Sprintf("db-%d-%d", idx, j)
				m.SetDatabase(dbName)
				// Just verify that we can get some database name, not a specific one,
				// since concurrent access means the last writer's value is not guaranteed
				got := m.GetDatabase()
				require.NotEmpty(t, got)
			}
		}(i)
	}
	wg.Wait()

	// Verify final state is consistent
	final := m.GetDatabase()
	require.NotEmpty(t, final)
}

func TestManager_GetDatabase(t *testing.T) {
	t.Parallel()

	t.Run("returns empty string by default", func(t *testing.T) {
		m := NewManager(&config.Prest{})
		got := m.GetDatabase()
		require.Equal(t, "", got)
	})

	t.Run("returns set database name", func(t *testing.T) {
		m := testManager(t)
		m.SetDatabase("custom-db")
		got := m.GetDatabase()
		require.Equal(t, "custom-db", got)
	})

	t.Run("returns latest set database on concurrent access", func(t *testing.T) {
		m := testManager(t)
		const setters = 20

		var wg sync.WaitGroup
		wg.Add(setters + 1)

		ready := make(chan struct{})
		var readyOnce sync.Once

		// Writers
		for i := range setters {
			go func(idx int) {
				defer wg.Done()
				m.SetDatabase(fmt.Sprintf("db-%d", idx))
				readyOnce.Do(func() { close(ready) })
			}(i)
		}

		// Concurrent reader - wait for at least one write, then verify valid results without race
		results := make([]string, 100)
		go func() {
			defer wg.Done()
			<-ready
			for i := range 100 {
				results[i] = m.GetDatabase()
			}
		}()

		wg.Wait()

		// All read values should be valid database names set by one of the writers
		for _, val := range results {
			require.NotEmpty(t, val)
			require.Regexp(t, `^db-\d+$`, val)
		}
	})
}

func TestGetURI(t *testing.T) {
	t.Parallel()

	t.Run("uses legacy database name when no profile", func(t *testing.T) {
		m := testManager(t)
		uri := m.GetURI("customdb")

		require.Contains(t, uri, "dbname=customdb")
		require.Contains(t, uri, "user=u")
		require.Contains(t, uri, "password=secret")
		require.Contains(t, uri, "host=localhost")
		require.Contains(t, uri, "port=5432")
	})

	t.Run("uses default database when name is empty", func(t *testing.T) {
		m := testManager(t)
		uri := m.GetURI("")

		require.Contains(t, uri, "dbname=testdb")
	})

	t.Run("includes all SSL parameters when set", func(t *testing.T) {
		cfg := &config.Prest{
			PGDatabase:    "testdb",
			PGHost:        "localhost",
			PGPort:        5432,
			PGUser:        "user",
			PGSSLMode:     "require",
			PGConnTimeout: 10,
			PGSSLCert:     "/path/to/cert.crt",
			PGSSLKey:      "/path/to/key.key",
			PGSSLRootCert: "/path/to/root.crt",
		}
		m := NewManager(cfg)
		uri := m.GetURI("testdb")

		require.Contains(t, uri, "sslcert=/path/to/cert.crt")
		require.Contains(t, uri, "sslkey=/path/to/key.key")
		require.Contains(t, uri, "sslrootcert=/path/to/root.crt")
	})

	t.Run("omits password when empty", func(t *testing.T) {
		cfg := &config.Prest{
			PGDatabase:    "testdb",
			PGHost:        "localhost",
			PGPort:        5432,
			PGUser:        "user",
			PGPass:        "",
			PGSSLMode:     "disable",
			PGConnTimeout: 10,
		}
		m := NewManager(cfg)
		uri := m.GetURI("testdb")

		require.NotContains(t, uri, "password=")
	})
}

func TestManager_Get(t *testing.T) {
	t.Run("returns existing database from pool", func(t *testing.T) {
		m := testManager(t)
		uri := m.GetURI("testdb")

		mockDB, _, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = mockDB.Close() })

		sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
		m.InjectDBForTest(uri, sqlxDB)
		m.SetDatabase("testdb")

		got, err := m.Get()
		require.NoError(t, err)
		require.Same(t, sqlxDB, got)
	})

	t.Run("returns error when current database not in pool and connect fails", func(t *testing.T) {
		m := testManager(t)
		m.SetDatabase("testdb")

		origConnect := dbConnect
		dbConnect = func(driverName, dataSourceName string) (*sqlx.DB, error) {
			return nil, fmt.Errorf("connection failed")
		}
		t.Cleanup(func() { dbConnect = origConnect })

		_, err := m.Get()
		require.Error(t, err)
		require.Contains(t, err.Error(), "connection failed")
	})

	t.Run("uses empty database name when current database is empty", func(t *testing.T) {
		m := testManager(t)
		uri := m.GetURI("")

		mockDB, _, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = mockDB.Close() })

		sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
		m.InjectDBForTest(uri, sqlxDB)

		got, err := m.Get()
		require.NoError(t, err)
		require.Same(t, sqlxDB, got)
	})
}

func TestManager_GetPool(t *testing.T) {
	t.Parallel()

	t.Run("returns initialized pool", func(t *testing.T) {
		m := NewManager(&config.Prest{})
		pool := m.GetPool()

		require.NotNil(t, pool)
		require.NotNil(t, pool.DB)
		require.Equal(t, 0, len(pool.DB))
	})

	t.Run("returns same pool on multiple calls", func(t *testing.T) {
		m := NewManager(&config.Prest{})
		pool1 := m.GetPool()
		pool2 := m.GetPool()

		require.Same(t, pool1, pool2)
	})
}

func TestManager_MustGet(t *testing.T) {
	t.Run("returns database when available", func(t *testing.T) {
		m := testManager(t)
		uri := m.GetURI("testdb")

		mockDB, _, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = mockDB.Close() })

		sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
		m.InjectDBForTest(uri, sqlxDB)
		m.SetDatabase("testdb")

		got := m.MustGet()
		require.Same(t, sqlxDB, got)
	})

	t.Run("panics when connection fails", func(t *testing.T) {
		m := testManager(t)
		m.SetDatabase("testdb")

		origConnect := dbConnect
		dbConnect = func(driverName, dataSourceName string) (*sqlx.DB, error) {
			return nil, fmt.Errorf("connection failed")
		}
		t.Cleanup(func() { dbConnect = origConnect })

		require.Panics(t, func() {
			m.MustGet()
		})
	})
}

func TestManager_CacheKeyForDB(t *testing.T) {
	t.Parallel()

	t.Run("returns empty string for nil db", func(t *testing.T) {
		m := testManager(t)
		key := m.CacheKeyForDB(nil)
		require.Equal(t, "", key)
	})

	t.Run("returns URI for db in pool", func(t *testing.T) {
		m := testManager(t)
		uri := m.GetURI("testdb")

		mockDB, _, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = mockDB.Close() })

		sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
		m.InjectDBForTest(uri, sqlxDB)

		key := m.CacheKeyForDB(sqlxDB)
		require.Equal(t, uri, key)
	})

	t.Run("returns pointer format for unknown db", func(t *testing.T) {
		m := testManager(t)

		mockDB, _, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = mockDB.Close() })

		sqlxDB := sqlx.NewDb(mockDB, "sqlmock")

		key := m.CacheKeyForDB(sqlxDB)
		require.NotEmpty(t, key)
		require.Contains(t, key, "0x")
	})

	t.Run("finds correct URI among multiple databases", func(t *testing.T) {
		m := testManager(t)
		uri1 := m.GetURI("db1")
		uri2 := m.GetURI("db2")

		mockDB1, _, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = mockDB1.Close() })

		mockDB2, _, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = mockDB2.Close() })

		sqlxDB1 := sqlx.NewDb(mockDB1, "sqlmock")
		sqlxDB2 := sqlx.NewDb(mockDB2, "sqlmock")

		m.InjectDBForTest(uri1, sqlxDB1)
		m.InjectDBForTest(uri2, sqlxDB2)

		key1 := m.CacheKeyForDB(sqlxDB1)
		key2 := m.CacheKeyForDB(sqlxDB2)

		require.Equal(t, uri1, key1)
		require.Equal(t, uri2, key2)
		require.NotEqual(t, key1, key2)
	})
}

func TestManager_RegisteredAliases(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when no registry configured", func(t *testing.T) {
		m := NewManager(&config.Prest{})
		aliases := m.RegisteredAliases()

		require.Nil(t, aliases)
	})

	t.Run("returns empty slice when Databases is empty", func(t *testing.T) {
		m := NewManager(&config.Prest{
			Databases: []config.DatabaseConf{},
		})
		aliases := m.RegisteredAliases()

		require.Empty(t, aliases)
	})

	t.Run("returns all aliases when configured", func(t *testing.T) {
		m := NewManager(&config.Prest{
			Databases: []config.DatabaseConf{
				{Alias: "prod"},
				{Alias: "staging"},
				{Alias: "dev"},
			},
		})
		aliases := m.RegisteredAliases()

		require.Len(t, aliases, 3)
		require.Contains(t, aliases, "prod")
		require.Contains(t, aliases, "staging")
		require.Contains(t, aliases, "dev")
	})

	t.Run("maintains alias order", func(t *testing.T) {
		m := NewManager(&config.Prest{
			Databases: []config.DatabaseConf{
				{Alias: "first"},
				{Alias: "second"},
				{Alias: "third"},
			},
		})
		aliases := m.RegisteredAliases()

		require.Equal(t, []string{"first", "second", "third"}, aliases)
	})
}

// The configured session zone travels as the lib/pq `timezone` startup
// parameter in key/value DSNs; the entry value wins over the global default.
func TestBuildURI_Timezone(t *testing.T) {
	t.Parallel()

	base := "user=u dbname=db host=h port=5432 sslmode=disable connect_timeout=10"
	tests := []struct {
		name     string
		conf     config.DatabaseConf
		defaults *config.Prest
		want     string
	}{
		{
			name:     "entry timezone",
			conf:     config.DatabaseConf{Host: "h", Port: 5432, User: "u", Database: "db", Timezone: "America/Sao_Paulo"},
			defaults: &config.Prest{PGSSLMode: "disable", PGConnTimeout: 10},
			want:     base + " timezone=America/Sao_Paulo",
		},
		{
			name:     "falls back to default timezone",
			conf:     config.DatabaseConf{Host: "h", Port: 5432, User: "u", Database: "db"},
			defaults: &config.Prest{PGSSLMode: "disable", PGConnTimeout: 10, PGTimezone: "America/Sao_Paulo"},
			want:     base + " timezone=America/Sao_Paulo",
		},
		{
			name:     "entry wins over default",
			conf:     config.DatabaseConf{Host: "h", Port: 5432, User: "u", Database: "db", Timezone: "America/Sao_Paulo"},
			defaults: &config.Prest{PGSSLMode: "disable", PGConnTimeout: 10, PGTimezone: "UTC"},
			want:     base + " timezone=America/Sao_Paulo",
		},
		{
			name: "timezone follows password and ssl keys",
			conf: config.DatabaseConf{
				Host: "h", Port: 5432, User: "u", Pass: "pw", Database: "db", Timezone: "UTC",
				SSL: config.DatabaseSSLConf{RootCert: "/r"},
			},
			defaults: &config.Prest{PGSSLMode: "disable", PGConnTimeout: 10},
			want:     base + " password=pw sslrootcert=/r timezone=UTC",
		},
		{
			name:     "unset timezone emits no key",
			conf:     config.DatabaseConf{Host: "h", Port: 5432, User: "u", Database: "db"},
			defaults: &config.Prest{PGSSLMode: "disable", PGConnTimeout: 10},
			want:     base,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, BuildURI(tt.conf, tt.defaults))
		})
	}
}

// URL DSNs get any user-supplied timezone key (any case) replaced by the
// configured zone; everything else in the URL, including an escaped
// password, survives. Without a zone or a timezone key the input is untouched.
func TestBuildURI_URLTimezone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		entryTZ  string
		globalTZ string
		want     string
	}{
		{
			name:    "adds configured zone",
			url:     "postgres://user:p%40ss@localhost:5432/db?sslmode=disable&connect_timeout=5",
			entryTZ: "America/Sao_Paulo",
			want:    "postgres://user:p%40ss@localhost:5432/db?sslmode=disable&connect_timeout=5&timezone=America%2FSao_Paulo",
		},
		{
			name:    "other pairs kept byte-for-byte in their order",
			url:     "postgres://u@h/db?application_name=my%20app&TimeZone=Foo&b=1&a=2",
			entryTZ: "UTC",
			want:    "postgres://u@h/db?application_name=my%20app&b=1&a=2&timezone=UTC",
		},
		{
			name:     "default zone used when entry is empty",
			url:      "postgresql://user:p%40ss@localhost/db",
			globalTZ: "America/Sao_Paulo",
			want:     "postgresql://user:p%40ss@localhost/db?timezone=America%2FSao_Paulo",
		},
		{
			name: "strips user timezone when no zone configured",
			url:  "postgres://user:p%40ss@localhost/db?TimeZone=Foo/Bar&sslmode=require",
			want: "postgres://user:p%40ss@localhost/db?sslmode=require",
		},
		{
			name:     "replaces every user timezone key with exactly one configured",
			url:      "postgres://u@h/db?timezone=Foo&TIMEZONE=Bar&TimeZone=Baz&connect_timeout=3",
			entryTZ:  "America/Sao_Paulo",
			globalTZ: "UTC",
			want:     "postgres://u@h/db?connect_timeout=3&timezone=America%2FSao_Paulo",
		},
		{
			name:    "scheme match is case-insensitive",
			url:     "POSTGRES://u@h/db",
			entryTZ: "UTC",
			want:    "postgres://u@h/db?timezone=UTC",
		},
		{
			name: "unset zone and no timezone key returns exact input",
			url:  "postgres://user:p%21ss@localhost/db?sslmode=disable&b=1&a=2",
			want: "postgres://user:p%21ss@localhost/db?sslmode=disable&b=1&a=2",
		},
		{
			name:    "non-postgres scheme returned verbatim",
			url:     "mysql://u@h/db?timezone=Foo",
			entryTZ: "UTC",
			want:    "mysql://u@h/db?timezone=Foo",
		},
		{
			name:    "key/value string in URL field returned verbatim",
			url:     "host=h user=u dbname=db",
			entryTZ: "UTC",
			want:    "host=h user=u dbname=db",
		},
		{
			name:    "unparsable URL returned verbatim",
			url:     "postgres://u@h:bad port/db",
			entryTZ: "UTC",
			want:    "postgres://u@h:bad port/db",
		},
		// A malformed query pair must not let a raw timezone key slip through:
		// lib/pq drops only the bad pair and would still apply the rest.
		{
			name: "malformed query: user timezone stripped when no zone configured",
			url:  "postgres://u:p@h/db?a=%zz&TimeZone=Foo/Bar",
			want: "postgres://u:p@h/db?a=%zz",
		},
		{
			name:    "malformed query: user timezone replaced by configured zone",
			url:     "postgres://u:p@h/db?a=%zz&TimeZone=Foo/Bar&sslmode=disable",
			entryTZ: "America/Sao_Paulo",
			want:    "postgres://u:p@h/db?a=%zz&sslmode=disable&timezone=America%2FSao_Paulo",
		},
		{
			name: "malformed query: escaped timezone key stripped",
			url:  "postgres://u@h/db?%74ime%5Aone=Foo&a=%zz&TIMEZONE=Bar",
			want: "postgres://u@h/db?a=%zz",
		},
		{
			name:    "malformed query: invalid timezone value replaced",
			url:     "postgres://u@h/db?timezone=%zz",
			entryTZ: "UTC",
			want:    "postgres://u@h/db?timezone=UTC",
		},
		{
			name:     "malformed query alone gets the configured zone appended",
			url:      "postgres://u@h/db?a=%zz&sslmode=disable",
			globalTZ: "UTC",
			want:     "postgres://u@h/db?a=%zz&sslmode=disable&timezone=UTC",
		},
		{
			name: "malformed query alone with zone unset returned unchanged",
			url:  "postgres://u@h/db?a=%zz&sslmode=disable",
			want: "postgres://u@h/db?a=%zz&sslmode=disable",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			conf := config.DatabaseConf{URL: tt.url, Timezone: tt.entryTZ}
			defaults := &config.Prest{PGTimezone: tt.globalTZ}
			got := BuildURI(conf, defaults)
			require.Equal(t, tt.want, got)
			requireOnlyConfiguredTimezone(t, got, tt.entryTZ, tt.globalTZ)
		})
	}
}

// requireOnlyConfiguredTimezone parses a rewritten postgres URL the way lib/pq
// does (url.Parse + Query) and requires that the only session timezone it
// would send is the configured one, or none when no zone is configured.
func requireOnlyConfiguredTimezone(t *testing.T, dsn, entryTZ, globalTZ string) {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return
	}
	tz := entryTZ
	if tz == "" {
		tz = globalTZ
	}
	var got []string
	for key, vals := range u.Query() {
		if strings.EqualFold(key, "timezone") {
			got = append(got, vals...)
		}
	}
	if tz == "" {
		require.Empty(t, got, dsn)
		return
	}
	require.Equal(t, []string{tz}, got, dsn)
}

// The legacy single-database DSN carries the global zone only when it is set.
func TestGetURI_Timezone(t *testing.T) {
	t.Parallel()

	cfg := &config.Prest{
		PGDatabase: "testdb", PGHost: "localhost", PGPort: 5432, PGUser: "u",
		PGPass: "secret", PGSSLMode: "disable", PGConnTimeout: 10,
		PGSSLRootCert: "/r", PGTimezone: "America/Sao_Paulo",
	}
	require.Equal(t,
		"user=u dbname=testdb host=localhost port=5432 sslmode=disable connect_timeout=10 password=secret sslrootcert=/r timezone=America/Sao_Paulo",
		NewManager(cfg).GetURI(""))

	require.NotContains(t, testManager(t).GetURI(""), "timezone")
}

// Manager.Get hands the connector a DSN that carries the configured zone, for
// both a registry alias and the legacy database.
func TestManager_Get_passesTimezoneToConnector(t *testing.T) {
	var (
		mu   sync.Mutex
		dsns []string
	)
	origConnect := dbConnect
	dbConnect = func(_, dataSourceName string) (*sqlx.DB, error) {
		mu.Lock()
		dsns = append(dsns, dataSourceName)
		mu.Unlock()
		mockDB, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return sqlx.NewDb(mockDB, "sqlmock"), nil
	}
	t.Cleanup(func() { dbConnect = origConnect })

	m := NewManager(&config.Prest{
		PGDatabase: "legacydb", PGHost: "localhost", PGPort: 5432, PGUser: "u",
		PGSSLMode: "disable", PGTimezone: "UTC",
		Databases: []config.DatabaseConf{
			{Alias: "tenant-a", Host: "h", Database: "a", Timezone: "America/Sao_Paulo"},
			{Alias: "tenant-b", URL: "postgres://u@h/b?TimeZone=Foo"},
		},
	})
	t.Cleanup(m.CloseAllAndResetPool)

	for _, name := range []string{"tenant-a", "tenant-b", "legacydb"} {
		m.SetDatabase(name)
		_, err := m.Get()
		require.NoError(t, err)
	}

	require.Len(t, dsns, 3)
	require.Contains(t, dsns[0], " timezone=America/Sao_Paulo")
	require.Equal(t, "postgres://u@h/b?timezone=UTC", dsns[1])
	require.Contains(t, dsns[2], "dbname=legacydb")
	require.Contains(t, dsns[2], " timezone=UTC")
}
