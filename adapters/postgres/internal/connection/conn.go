package connection

import (
	"fmt"
	"log"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/config"

	// Used pg drive on sqlx
	_ "github.com/lib/pq"
)

var (
	err          error
	pool         Pool = Pool{DB: make(map[string]*sqlx.DB)}
	currDatabase string
	connectMtx   sync.Mutex
)

// Pool struct
type Pool struct {
	Mtx sync.Mutex
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

	log.Println(fmt.Sprintf("The dbURI is: %s", dbURI))

	return dbURI
}

// Get get postgres connection
func Get(database string) (*sqlx.DB, error) {
	var DB *sqlx.DB

	if database == "" {
		database = GetDatabase()
	}
	DB = getDatabaseFromPool(database)
	if DB != nil {
		return DB, nil
	}

	connectMtx.Lock()
	DB, err = sqlx.Connect("postgres", GetURI(database))
	connectMtx.Unlock()
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
	return &pool
}

func getDatabaseFromPool(name string) *sqlx.DB {
	var DB *sqlx.DB
	var p *Pool

	p = GetPool()

	p.Mtx.Lock()
	DB = p.DB[GetURI(name)]
	p.Mtx.Unlock()

	return DB
}

// AddDatabaseToPool add connection to pool
func AddDatabaseToPool(name string, DB *sqlx.DB) {
	var p *Pool

	p = GetPool()

	p.Mtx.Lock()
	p.DB[GetURI(name)] = DB
	p.Mtx.Unlock()
}

// MustGet get postgres connection
func MustGet() *sqlx.DB {
	var err error
	var DB *sqlx.DB

	DB, err = Get("") // TODO
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v\n", err))
	}
	return DB
}

// SetDatabase set current database in use
func SetDatabase(name string) {
	p := GetPool()
	p.Mtx.Lock()
	currDatabase = name
	p.Mtx.Unlock()
}

// GetDatabase get current database in use
func GetDatabase() (result string) {
	p := GetPool()
	p.Mtx.Lock()
	result = currDatabase
	p.Mtx.Unlock()

	return
}
