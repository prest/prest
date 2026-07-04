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

func (m *Manager) hasRegistry() bool {
	return config.HasDatabaseRegistry(m.cfg)
}

// GetURI postgres connection URI for alias or legacy database name.
func (m *Manager) GetURI(name string) string {
	if conf, ok := config.ProfileByAlias(m.cfg, name); ok {
		return BuildURI(conf, m.cfg)
	}

	dbName := name
	if dbName == "" {
		dbName = m.cfg.PGDatabase
	}
	dbURI := fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=%v connect_timeout=%d",
		m.cfg.PGUser,
		dbName,
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

// BuildURI builds a postgres connection URI from a database profile.
func BuildURI(conf config.DatabaseConf, defaults *config.Prest) string {
	if conf.URL != "" {
		return conf.URL
	}

	dbName := conf.Database
	if dbName == "" {
		dbName = defaults.PGDatabase
	}
	port := conf.Port
	if port == 0 {
		port = defaults.PGPort
	}
	sslMode := conf.SSL.Mode
	if sslMode == "" {
		sslMode = defaults.PGSSLMode
	}
	user := conf.User
	if user == "" {
		user = defaults.PGUser
	}
	host := conf.Host
	if host == "" {
		host = defaults.PGHost
	}

	dbURI := fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=%v connect_timeout=%d",
		user,
		dbName,
		host,
		port,
		sslMode,
		defaults.PGConnTimeout,
	)
	if conf.Pass != "" {
		dbURI += " password=" + conf.Pass
	} else if defaults.PGPass != "" {
		dbURI += " password=" + defaults.PGPass
	}
	if conf.SSL.Cert != "" {
		dbURI += " sslcert=" + conf.SSL.Cert
	}
	if conf.SSL.Key != "" {
		dbURI += " sslkey=" + conf.SSL.Key
	}
	if conf.SSL.RootCert != "" {
		dbURI += " sslrootcert=" + conf.SSL.RootCert
	}
	return dbURI
}

// Get gets a Postgres connection adding it to the pool if needed
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

// GetPool gets the connection pool
func (m *Manager) GetPool() *Pool {
	return m.getPool()
}

// poolLimitsFor returns the maximum number of idle and open connections for
// a given database. It uses the global limits if the database is not found in
// the registry.
func (m *Manager) poolLimitsFor(name string) (maxIdle, maxOpen int) {
	maxIdle = m.cfg.PGMaxIdleConn
	maxOpen = m.cfg.PGMaxOpenConn
	if conf, ok := config.ProfileByAlias(m.cfg, name); ok {
		if conf.MaxIdleConn != 0 {
			maxIdle = conf.MaxIdleConn
		}
		if conf.MaxOpenConn != 0 {
			maxOpen = conf.MaxOpenConn
		}
	}
	return maxIdle, maxOpen
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
		maxIdle, maxOpen := m.poolLimitsFor(name)
		DB.SetMaxIdleConns(maxIdle)
		DB.SetMaxOpenConns(maxOpen)

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

// SetDatabase sets the current database in use
func (m *Manager) SetDatabase(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currDatabase = name
}

// GetDatabase gets the current database in use
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

// RegisteredAliases returns configured database aliases when a registry is active.
func (m *Manager) RegisteredAliases() []string {
	if !m.hasRegistry() {
		return nil
	}
	aliases := make([]string, len(m.cfg.Databases))
	for i, db := range m.cfg.Databases {
		aliases[i] = db.Alias
	}
	return aliases
}
