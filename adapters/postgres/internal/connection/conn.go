package connection

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/internal/logsafe"

	"github.com/jmoiron/sqlx"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"
	"golang.org/x/sync/singleflight"
)

// Pool struct
type Pool struct {
	Mtx *sync.RWMutex
	DB  map[string]*sqlx.DB
}

// Manager holds connection pool state for a single config instance.
type Manager struct {
	cfg          *config.Prest
	mu           sync.RWMutex
	pool         *Pool
	currDatabase string
	addDB        singleflight.Group
}

// dbConnect opens a database connection. Overridden in unit tests.
// do not use this function directly, use Get() instead
// nolint:revive
var dbConnect = sqlx.Connect

// NewManager creates a connection manager for the given config.
func NewManager(cfg *config.Prest) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) getPool() *Pool {
	m.mu.RLock()
	if m.pool != nil {
		pool := m.pool
		m.mu.RUnlock()
		return pool
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pool == nil {
		m.pool = &Pool{
			Mtx: &sync.RWMutex{},
			DB:  make(map[string]*sqlx.DB),
		}
	}
	return m.pool
}

// GetURI postgres connection URI
func (m *Manager) GetURI(DBName string) string {
	var dbURI string

	if DBName == "" {
		DBName = m.cfg.PGDatabase
	}
	dbURI = fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=%v connect_timeout=%d",
		m.cfg.PGUser,
		DBName,
		m.cfg.PGHost,
		m.cfg.PGPort,
		m.cfg.PGSSLMode,
		m.cfg.PGConnTimeout)

	if m.cfg.PGPass != "" {
		dbURI += " password=" + m.cfg.PGPass
	}
	if m.cfg.PGSSLCert != "" {
		dbURI += " sslcert=" + m.cfg.PGSSLCert
	}
	if m.cfg.PGSSLKey != "" {
		dbURI += " sslkey=" + m.cfg.PGSSLKey
	}
	if m.cfg.PGSSLRootCert != "" {
		dbURI += " sslrootcert=" + m.cfg.PGSSLRootCert
	}

	return dbURI
}

// Get get Postgres connection adding it to the pool if needed
func (m *Manager) Get() (*sqlx.DB, error) {
	DB := m.getDatabaseFromPool(m.GetDatabase())
	// Connection is already in the pool
	if DB != nil {
		return DB, nil
	}

	// Connection is not in the pool, add it
	DB, err := m.AddDatabaseToPool(m.GetDatabase())

	return DB, err
}

// GetFromPool tries to get the db name from the db pool
// will return error if not found
func (m *Manager) GetFromPool(dbName string) (*sqlx.DB, error) {
	DB := m.getDatabaseFromPool(dbName)
	if DB == nil {
		return nil, errors.New("db not found in pool")
	}
	return DB, nil
}

// GetPool of connection
func (m *Manager) GetPool() *Pool {
	return m.getPool()
}

func (m *Manager) getDatabaseFromPool(name string) *sqlx.DB {
	uri := m.GetURI(name)
	p := m.getPool()

	p.Mtx.RLock()
	DB := p.DB[uri]
	p.Mtx.RUnlock()

	return DB
}

// AddDatabaseToPool create and add connection to the pool
func (m *Manager) AddDatabaseToPool(name string) (*sqlx.DB, error) {
	if DB := m.getDatabaseFromPool(name); DB != nil {
		return DB, nil
	}

	uri := m.GetURI(name)
	result, err, _ := m.addDB.Do(uri, func() (interface{}, error) {
		if DB := m.getDatabaseFromPool(name); DB != nil {
			return DB, nil
		}

		DB, err := dbConnect("postgres", uri)
		if err != nil {
			return nil, err
		}
		DB.SetMaxIdleConns(m.cfg.PGMaxIdleConn)
		DB.SetMaxOpenConns(m.cfg.PGMaxOpenConn)

		p := m.getPool()
		p.Mtx.Lock()
		p.DB[uri] = DB
		p.Mtx.Unlock()
		return DB, nil
	})
	if err != nil {
		return nil, err
	}
	DB, ok := result.(*sqlx.DB)
	if !ok {
		return nil, errors.New("unexpected connection pool result")
	}
	return DB, nil
}

// MustGet get postgres connection
func (m *Manager) MustGet() *sqlx.DB {
	var err error
	var DB *sqlx.DB

	DB, err = m.Get()
	if err != nil {
		safeErr := logsafe.Error(err)
		slog.Error("Unable to connect to database", "error", safeErr)
		panic(safeErr)
	}
	return DB
}

// SetDatabase set current database in use
func (m *Manager) SetDatabase(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currDatabase = name
}

// GetDatabase get current database in use
func (m *Manager) GetDatabase() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currDatabase
}

// CacheKeyForDB returns a stable cache key for the given pool connection.
func (m *Manager) CacheKeyForDB(db *sqlx.DB) string {
	if db == nil {
		return ""
	}
	p := m.getPool()
	p.Mtx.RLock()
	defer p.Mtx.RUnlock()
	for uri, poolDB := range p.DB {
		if poolDB == db {
			return uri
		}
	}
	return fmt.Sprintf("%p", db)
}
