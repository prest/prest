package connection

import (
	"fmt"

	"database/sql"

	"github.com/jmoiron/sqlx"
	"github.com/prest/config"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"
)

var (
	// DB connection
	DB  *sqlx.DB
	err error
)

// Get get postgres connection
func Get() (*sqlx.DB, error) {
	if DB == nil {
		dbURI := fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=disable connect_timeout=%d",
			config.PrestConf.PGUser,
			config.PrestConf.PGDatabase,
			config.PrestConf.PGHost,
			config.PrestConf.PGPort,
			config.PrestConf.PGConnTimeout)
		if config.PrestConf.PGPass != "" {
			dbURI += " password=" + config.PrestConf.PGPass
		}
		DB, err = sqlx.Connect("postgres", dbURI)
		if err != nil {
			return nil, err
		}
		DB.SetMaxIdleConns(config.PrestConf.PGMaxIdleConn)
		DB.SetMaxOpenConns(config.PrestConf.PGMAxOpenConn)
	}
	return DB, nil
}

// MustGet get postgres connection
func MustGet() *sqlx.DB {
	var err error
	DB, err = Get()
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v\n", err))
	}
	return DB
}

// SetNativeDB enable to override sqlx native db
func SetNativeDB(native *sql.DB) {
	DB.DB = native
}
