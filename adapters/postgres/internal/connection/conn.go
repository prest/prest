package connection

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/prest/prest/v2/config"

	"github.com/jmoiron/sqlx"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"
)

// Pool struct
type Pool struct {
	Mtx *sync.Mutex
	DB  map[string]*sqlx.DB
}

// Manager holds connection pool state for a single config instance.
type Manager struct {
	cfg          *config.Prest
	pool         *Pool
	currDatabase string
}

// NewManager creates a connection manager for the given config.
func NewManager(cfg *config.Prest) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) getPool() *Pool {
	if m.pool == nil {
		m.pool = &Pool{
			Mtx: &sync.Mutex{},
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
	var DB *sqlx.DB
	p := m.getPool()

	p.Mtx.Lock()
	DB = p.DB[m.GetURI(name)]
	p.Mtx.Unlock()

	return DB
}

// AddDatabaseToPool create and add connection to the pool
func (m *Manager) AddDatabaseToPool(name string) (*sqlx.DB, error) {
	DB, err := sqlx.Connect("postgres", m.GetURI(name))
	if err != nil {
		return nil, err
	}
	DB.SetMaxIdleConns(m.cfg.PGMaxIdleConn)
	DB.SetMaxOpenConns(m.cfg.PGMaxOpenConn)

	p := m.getPool()

	p.Mtx.Lock()
	p.DB[m.GetURI(name)] = DB
	p.Mtx.Unlock()
	return DB, nil
}

// MustGet get postgres connection
func (m *Manager) MustGet() *sqlx.DB {
	var err error
	var DB *sqlx.DB

	DB, err = m.Get()
	if err != nil {
		slog.Error("Unable to connect to database", "error", err)
		panic(err)
	}
	return DB
}

// SetDatabase set current database in use
func (m *Manager) SetDatabase(name string) {
	m.currDatabase = name
}

// GetDatabase get current database in use
func (m *Manager) GetDatabase() string {
	return m.currDatabase
}
