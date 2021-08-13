package connection

import (
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/config"

	// Used pg drive on sqlx
	_ "github.com/lib/pq"
)

var (
	err          error
	pool         *Pool
)

// Pool struct
type Pool struct {
	Mtx *sync.Mutex
	DB  map[string]*sqlx.DB
}

// GetURI postgres connection URI
func GetURI(DBName string) string {
	if DBName == "" {
		DBName = config.PrestConf.PGDatabase
	}
	dbURI := fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=%v connect_timeout=%d",
		config.PrestConf.PGUser,
		DBName,
		config.PrestConf.PGHost,
		config.PrestConf.PGPort,
		config.PrestConf.SSLMode,
		config.PrestConf.PGConnTimeout)

	if config.PrestConf.PGPass != "" {
		dbURI += " password=" + config.PrestConf.PGPass
	}
	if config.PrestConf.SSLCert != "" {
		dbURI += " sslcert=" + config.PrestConf.SSLCert
	}
	if config.PrestConf.SSLKey != "" {
		dbURI += " sslkey=" + config.PrestConf.SSLKey
	}
	if config.PrestConf.SSLRootCert != "" {
		dbURI += " sslrootcert=" + config.PrestConf.SSLRootCert
	}

	return dbURI
}

// Get get postgres connection
func Get(database string) (*sqlx.DB, error) {
	DB := getDatabaseFromPool(database)
	if DB != nil {
		return DB, nil
	}

	DB, err = sqlx.Connect("postgres", GetURI(database))
	if err != nil {
		return nil, err
	}
	DB.SetMaxIdleConns(config.PrestConf.PGMaxIdleConn)
	DB.SetMaxOpenConns(config.PrestConf.PGMAxOpenConn)

	AddDatabaseToPool(database, DB)

	return DB, nil
}

// GetPool of connection
func GetPool() *Pool {
	if pool == nil {
		pool = &Pool{
			Mtx: &sync.Mutex{},
			DB:  make(map[string]*sqlx.DB),
		}
	}
	return pool
}

func getDatabaseFromPool(name string) *sqlx.DB {
	p := GetPool()

	p.Mtx.Lock()
	DB := p.DB[GetURI(name)]
	p.Mtx.Unlock()

	return DB
}

// AddDatabaseToPool add connection to pool
func AddDatabaseToPool(name string, DB *sqlx.DB) {
	p := GetPool()

	p.Mtx.Lock()
	p.DB[GetURI(name)] = DB
	p.Mtx.Unlock()
}

// MustGet get postgres connection
func MustGet(database string) *sqlx.DB {
	DB, err := Get(database)
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v\n", err))
	}
	return DB
}
