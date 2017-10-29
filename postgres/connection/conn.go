package connection

import (
	"fmt"
	"sync"

	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/prest/config"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"
)

var (
	err error
	pool *ConnectionPool
	currDatabase string
)

type ConnectionPool struct {
	Mtx	*sync.Mutex
	DB	map[string]*sqlx.DB
}

// GetURI postgres connection URI
func GetURI(DBName string) string {
	var dbURI string

	if DBName == "" {
		DBName = config.PrestConf.PGDatabase
	}
	dbURI = fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=disable connect_timeout=%d",
		config.PrestConf.PGUser,
		DBName,
		config.PrestConf.PGHost,
		config.PrestConf.PGPort,
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
func Get() (*sqlx.DB, error) {
	var DB	*sqlx.DB

	DB = getDatabaseFromPool(GetDatabase())
	if DB != nil {
		return DB, nil
	}

	DB, err = sqlx.Connect("postgres", GetURI(GetDatabase()))
	if err != nil {
		return nil, err
	}
	DB.SetMaxIdleConns(config.PrestConf.PGMaxIdleConn)
	DB.SetMaxOpenConns(config.PrestConf.PGMAxOpenConn)

	addDatabaseToPool(GetDatabase(), DB)

	return DB, nil
}

func GetPool() *ConnectionPool {
	if pool == nil {
		pool = &ConnectionPool {
			Mtx:	&sync.Mutex{},
			DB:		make(map[string]*sqlx.DB),
		}
	}
	return pool
}

func getDatabaseFromPool(name string) *sqlx.DB {
	var DB	*sqlx.DB
	var p	*ConnectionPool

	p = GetPool()

	p.Mtx.Lock()
	DB = p.DB[GetURI(name)]
	p.Mtx.Unlock()

	return DB
}

func addDatabaseToPool(name string, DB *sqlx.DB) {
	var p	*ConnectionPool

	p = GetPool()

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

// SetNativeDB enable to override sqlx native db
func SetNativeDB(native *sql.DB) {
	var DB *sqlx.DB

	DB = getDatabaseFromPool(GetDatabase())
	DB.DB = native
	addDatabaseToPool("__native", DB)
}

// SetDatabase set current database in use
func SetDatabase(name string) {
	currDatabase = name
}

// GetDatabase get current database in use
func GetDatabase() string {
	return currDatabase
}
