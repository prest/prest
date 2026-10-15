package connection

import (
	"fmt"
	"slices"
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
// It tracks the active alias and never issues USE. It keeps one pool per DSN:
// pg.database (or the registry entry), plus, without a registry, one per
// pg.allowed_databases name on the same host. Pools open lazily on first use,
// so at most 1+len(AllowedDatabases) exist; unowned names never dial.
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
		m.cfg.MySQLPrepare,
	)
}

// Owns reports whether name is this manager's alias or physical database,
// or, without a registry, one of pg.allowed_databases.
// An empty name refers to the current connection.
func (m *Manager) Owns(name string) bool {
	if name == "" {
		return true
	}
	if name == m.GetDatabase() {
		return true
	}
	if m.cfg == nil {
		return false
	}
	if name == m.cfg.PGDatabase {
		return true
	}
	return !m.cfg.HasDatabaseRegistry() && slices.Contains(m.cfg.AllowedDatabases, name)
}

// dsnFor returns the DSN serving name. The empty name, the alias and
// pg.database use DSN(); any other owned name is an allow-listed database on
// the same [pg] host and credentials, with only the database swapped.
func (m *Manager) dsnFor(name string) (string, error) {
	if m.cfg == nil {
		return "", fmt.Errorf("mysql config is nil")
	}
	if name == "" || name == m.GetDatabase() || name == m.cfg.PGDatabase {
		return m.DSN()
	}
	return BuildDSN(
		m.cfg.PGUser,
		m.cfg.PGPass,
		m.cfg.PGHost,
		m.cfg.PGPort,
		name,
		m.cfg.PGSSLMode,
		m.cfg.PGSSLCert,
		m.cfg.PGSSLKey,
		m.cfg.PGSSLRootCert,
		m.cfg.MySQLPrepare,
	)
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

// GetOwned returns the pool for an owned name, opening it on first use so
// lazy adapters recover once the database is reachable. Names this manager
// does not own are rejected without dialling.
func (m *Manager) GetOwned(name string) (*sqlx.DB, error) {
	if !m.Owns(name) {
		return nil, fmt.Errorf("unknown database alias %q", name)
	}
	dsn, err := m.dsnFor(name)
	if err != nil {
		return nil, err
	}
	return m.open(dsn)
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
