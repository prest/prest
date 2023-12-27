package connection

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	// Use pg driver on sqlx
	_ "github.com/lib/pq"

	"github.com/prest/prest/config"
)

// Pool struct
type Pool struct {
	Mtx          *sync.Mutex
	DB           map[string]*sqlx.DB
	cfg          *config.Prest
	currDatabase string
}

func NewPool(cfg *config.Prest) *Pool {
	return &Pool{
		Mtx:          &sync.Mutex{},
		DB:           make(map[string]*sqlx.DB),
		cfg:          cfg,
		currDatabase: cfg.PGDatabase,
	}
}

// GetURI postgres connection URI
func (p *Pool) GetURI(DBName string) string {
	var dbURI string

	if DBName == "" {
		DBName = p.cfg.PGDatabase
	}
	dbURI = fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=%v connect_timeout=%d",
		p.cfg.PGUser,
		DBName,
		p.cfg.PGHost,
		p.cfg.PGPort,
		p.cfg.PGSSLMode,
		p.cfg.PGConnTimeout)

	if p.cfg.PGPass != "" {
		dbURI += " password=" + p.cfg.PGPass
	}
	if p.cfg.PGSSLCert != "" {
		dbURI += " sslcert=" + p.cfg.PGSSLCert
	}
	if p.cfg.PGSSLKey != "" {
		dbURI += " sslkey=" + p.cfg.PGSSLKey
	}
	if p.cfg.PGSSLRootCert != "" {
		dbURI += " sslrootcert=" + p.cfg.PGSSLRootCert
	}

	return dbURI
}

// Get get postgres connection
func (p *Pool) Get() (*sqlx.DB, error) {
	var (
		DB  *sqlx.DB
		err error
	)

	DB = p.getDatabaseFromPool(p.GetDatabase())
	if DB != nil {
		return DB, nil
	}

	DB, err = sqlx.Connect("postgres", p.GetURI(p.GetDatabase()))
	if err != nil {
		return nil, err
	}

	DB.SetMaxIdleConns(p.cfg.PGMaxIdleConn)
	DB.SetMaxOpenConns(p.cfg.PGMaxOpenConn)

	p.AddDatabaseToPool(p.GetDatabase(), DB.DB)

	return DB, nil
}

// GetFromPool tries to get the db name from the db pool
// will return error if not found
func (p *Pool) GetFromPool(dbName string) (*sqlx.DB, error) {
	DB := p.getDatabaseFromPool(dbName)
	if DB == nil {
		return nil, errors.New("db not found in pool")
	}
	return DB, nil
}

func (p *Pool) getDatabaseFromPool(name string) *sqlx.DB {
	var DB *sqlx.DB

	p.Mtx.Lock()
	DB = p.DB[p.GetURI(name)]
	p.Mtx.Unlock()

	return DB
}

// AddDatabaseToPool add connection to pool
func (p *Pool) AddDatabaseToPool(name string, DB *sql.DB) {
	db := sqlx.NewDb(DB, "postgres")
	p.Mtx.Lock()
	p.DB[p.GetURI(name)] = db
	p.Mtx.Unlock()
}

// MustGet get postgres connection
func (p *Pool) MustGet() *sqlx.DB {
	var err error
	var DB *sqlx.DB

	DB, err = p.Get()
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v\n", err))
	}
	return DB
}

// SetDatabase set current database in use
func (p *Pool) SetDatabase(name string) {
	p.Mtx.Lock()
	p.currDatabase = name
	p.Mtx.Unlock()
}

// GetDatabase get current database in use
func (p *Pool) GetDatabase() string {
	return p.currDatabase
}
