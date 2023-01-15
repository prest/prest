package connection

import (
	"errors"
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
	currDatabase string
)

// Pool struct
type Pool struct {
	Mtx *sync.Mutex
	DB  map[string]*sqlx.DB
}

// GetURI postgres connection URI
func GetURI(DBName string) string {
	var dbURI string

	if DBName == "" {
		DBName = config.PrestConf.PGDatabase
	}
	dbURI = fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=%v connect_timeout=%d",
		config.PrestConf.PGUser,
		DBName,
		config.PrestConf.PGHost,
		config.PrestConf.PGPort,
		config.PrestConf.PGSSLMode,
		config.PrestConf.PGConnTimeout)

	if config.PrestConf.PGPass != "" {
		dbURI += " password=" + config.PrestConf.PGPass
	}
	if config.PrestConf.PGSSLCert != "" {
		dbURI += " sslcert=" + config.PrestConf.PGSSLCert
	}
	if config.PrestConf.PGSSLKey != "" {
		dbURI += " sslkey=" + config.PrestConf.PGSSLKey
	}
	if config.PrestConf.PGSSLRootCert != "" {
		dbURI += " sslrootcert=" + config.PrestConf.PGSSLRootCert
	}

	return dbURI
}

// Get get postgres connection
func Get() (*sqlx.DB, error) {
	var DB *sqlx.DB

	DB = getDatabaseFromPool(GetDatabase())
	if DB != nil {
		return DB, nil
	}

	DB, err = sqlx.Connect("postgres", GetURI(GetDatabase()))
	if err != nil {
		return nil, err
	}
	DB.SetMaxIdleConns(config.PrestConf.PGMaxIdleConn)
	DB.SetMaxOpenConns(config.PrestConf.PGMaxOpenConn)

	AddDatabaseToPool(GetDatabase(), DB)

	return DB, nil
}

// GetFromPool tries to get the db name from the db pool
// will return error if not found
func GetFromPool(dbName string) (*sqlx.DB, error) {
	DB := getDatabaseFromPool(dbName)
	if DB == nil {
		return nil, errors.New("db not found in pool")
	}
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
	var DB *sqlx.DB
	p := GetPool()

	p.Mtx.Lock()
	DB = p.DB[GetURI(name)]
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
func MustGet() *sqlx.DB {
	var err error
	var DB *sqlx.DB

	DB, err = Get()
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v\n", err))
	}
	return DB
}

// SetDatabase set current database in use
func SetDatabase(name string) {
	currDatabase = name
}

// GetDatabase get current database in use
func GetDatabase() string {
	return currDatabase
}
