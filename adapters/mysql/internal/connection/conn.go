package connection

import (
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/v2/config"

	"golang.org/x/sync/singleflight"

	// MySQL driver. OTel wrapping is a later phase; v1 does not register a Postgres driver.
	_ "github.com/go-sql-driver/mysql"
)

const defaultDriverName = "mysql"

// Pool holds open connections keyed by DSN.
type Pool struct {
	Mtx *sync.RWMutex
	DB  map[string]*sqlx.DB
}

// Manager is the MySQL connection pool for one pREST config.
// It tracks the active alias. It does not issue USE, and it does not open a
// connection for a name it does not already own.
type Manager struct {
	cfg        *config.Prest
	mu         sync.RWMutex
	pool       *Pool
	alias      string
	addDB      singleflight.Group
	driverName string
}

// dbConnect opens a database connection. Overridden in unit tests.
var dbConnect = sqlx.Connect

// NewManager creates a connection manager for cfg.
func NewManager(cfg *config.Prest) *Manager {
	return &Manager{cfg: cfg, driverName: defaultDriverName}
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

// DSN builds the DSN for this manager's single connection.
func (m *Manager) DSN() (string, error) {
	if m.cfg == nil {
		return "", fmt.Errorf("mysql config is nil")
	}
	return BuildDSN(
		m.cfg.PGUser,
		m.cfg.PGPass,
		m.cfg.PGHost,
		m.cfg.PGPort,
		m.cfg.PGDatabase,
		m.cfg.PGSSLMode,
		m.cfg.PGSSLCert,
		m.cfg.PGSSLKey,
		m.cfg.PGSSLRootCert,
	)
}

// Owns reports whether name is this manager's alias or physical database.
// An empty name refers to the current connection.
func (m *Manager) Owns(name string) bool {
	if name == "" {
		return true
	}
	m.mu.RLock()
	alias := m.alias
	m.mu.RUnlock()
	if name == alias {
		return true
	}
	return m.cfg != nil && name == m.cfg.PGDatabase
}

// Get returns the pooled connection, opening it when this manager does not
// yet hold one.
func (m *Manager) Get() (*sqlx.DB, error) {
	dsn, err := m.DSN()
	if err != nil {
		return nil, err
	}
	if db := m.pooled(dsn); db != nil {
		return db, nil
	}
	return m.open(dsn)
}

// GetOwned returns a connection this manager already holds when name is owned.
// It does not open a connection for an unknown alias.
func (m *Manager) GetOwned(name string) (*sqlx.DB, error) {
	if !m.Owns(name) {
		return nil, fmt.Errorf("unknown database alias %q", name)
	}
	dsn, err := m.DSN()
	if err != nil {
		return nil, err
	}
	db := m.pooled(dsn)
	if db == nil {
		return nil, fmt.Errorf("unknown database alias %q", name)
	}
	return db, nil
}

func (m *Manager) pooled(dsn string) *sqlx.DB {
	p := m.getPool()
	p.Mtx.RLock()
	defer p.Mtx.RUnlock()
	return p.DB[dsn]
}

func (m *Manager) open(dsn string) (*sqlx.DB, error) {
	result, err, _ := m.addDB.Do(dsn, func() (interface{}, error) {
		if db := m.pooled(dsn); db != nil {
			return db, nil
		}
		db, err := dbConnect(m.driverName, dsn)
		if err != nil {
			return nil, fmt.Errorf("open mysql connection to %s: %w", RedactedDSN(dsn), err)
		}
		maxIdle, maxOpen := 0, 0
		if m.cfg != nil {
			maxIdle = m.cfg.PGMaxIdleConn
			maxOpen = m.cfg.PGMaxOpenConn
		}
		db.SetMaxIdleConns(maxIdle)
		db.SetMaxOpenConns(maxOpen)
		p := m.getPool()
		p.Mtx.Lock()
		p.DB[dsn] = db
		p.Mtx.Unlock()
		return db, nil
	})
	if err != nil {
		return nil, err
	}
	db, ok := result.(*sqlx.DB)
	if !ok {
		return nil, fmt.Errorf("unexpected mysql connection result")
	}
	return db, nil
}

// SetDatabase records the active alias. It does not change the MySQL session database.
func (m *Manager) SetDatabase(name string) {
	m.mu.Lock()
	m.alias = name
	m.mu.Unlock()
}

// GetDatabase returns the active alias.
func (m *Manager) GetDatabase() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alias
}

// CloseAll closes pooled connections.
func (m *Manager) CloseAll() {
	p := m.getPool()
	p.Mtx.Lock()
	for _, db := range p.DB {
		_ = db.Close()
	}
	p.DB = make(map[string]*sqlx.DB)
	p.Mtx.Unlock()
}
