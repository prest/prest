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

var (
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

// Get get Postgres connection adding it to the pool if needed
func Get() (*sqlx.DB, error) {
	DB := getDatabaseFromPool(GetDatabase())
	// Connection is already in the pool
	if DB != nil {
		return DB, nil
	}

	// Connection is not in the pool, add it
	DB, err := AddDatabaseToPool(GetDatabase())

	return DB, err
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

// AddDatabaseToPool create and add connection to the pool
func AddDatabaseToPool(name string) (*sqlx.DB, error) {
	DB, err := sqlx.Connect("postgres", GetURI(name))
	if err != nil {
		return nil, err
	}
	DB.SetMaxIdleConns(config.PrestConf.PGMaxIdleConn)
	DB.SetMaxOpenConns(config.PrestConf.PGMaxOpenConn)

	p := GetPool()

	p.Mtx.Lock()
	p.DB[GetURI(name)] = DB
	p.Mtx.Unlock()
	return DB, nil
}

// MustGet get postgres connection
func MustGet() *sqlx.DB {
	var err error
	var DB *sqlx.DB

	DB, err = Get()
	if err != nil {
		slog.Error("Unable to connect to database", "error", err)
		panic(err)
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
