package connection

import (
	"fmt"

	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/nuveo/prest/config"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"
)

var (
	db  *sqlx.DB
	err error
)

// MustGet get postgres connection
func MustGet() *sqlx.DB {
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
			panic(fmt.Sprintf("Unable to connection to database: %v\n", err))
		}
		db.SetMaxIdleConns(config.PrestConf.PGMaxIdleConn)
		db.SetMaxOpenConns(config.PrestConf.PGMAxOpenConn)
	}
	return db
}

// SetNativeDB enable to override sqlx native db
func SetNativeDB(native *sql.DB) {
	db.DB = native
}
