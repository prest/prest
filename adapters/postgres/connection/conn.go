package connection

import (
	"fmt"

	"database/sql"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/nuveo/prest/config"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"
)

var (
	db  *sqlx.DB
	err error
)

// Get get postgres connection
func Get() (*sqlx.DB, error) {
	if db == nil {
		dbURI := fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=disable connect_timeout=%d",
			config.PrestConf.PGUser,
			config.PrestConf.PGDatabase,
			config.PrestConf.PGHost,
			config.PrestConf.PGPort,
			config.PrestConf.PGConnTimeout)
		if config.PrestConf.PGPass != "" {
			dbURI += " password=" + config.PrestConf.PGPass
		}
		db, err = sqlx.Connect("postgres", dbURI)
		if err != nil {
			return nil, err
		}
		db.SetMaxIdleConns(config.PrestConf.PGMaxIdleConn)
		db.SetMaxOpenConns(config.PrestConf.PGMAxOpenConn)
	}
	return db, nil
}

// MustGet get postgres connection
func MustGet() *sqlx.DB {
	var err error
	db, err = Get()
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v\n", err))
	}
	return db
}

// SetNativeDB enable to override sqlx native db
func SetNativeDB(native *sql.DB) {
	db.DB = native
}

// UseMockDB mock database
func UseMockDB(driverName string) (mock sqlmock.Sqlmock, err error) {
	var nativeDB *sql.DB

	nativeDB, mock, err = sqlmock.New()
	if err != nil {
		return
	}
	db = sqlx.NewDb(nativeDB, driverName)
	return
}
