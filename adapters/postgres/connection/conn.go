package connection

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/nuveo/prest/config"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"
)

var (
	db  *sqlx.DB
	cfg config.Prest
	err error
)

// MustGet get postgre cpnnection
func MustGet() *sqlx.DB {
	if db == nil {
		cfg := config.Prest{}
		config.Parse(&cfg)
		dbURI := fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=disable", cfg.PGUser, cfg.PGDatabase, cfg.PGHost, cfg.PGPort)
		if cfg.PGPass != "" {
			dbURI += " password=" + cfg.PGPass
		}
		db, err = sqlx.Connect("postgres", dbURI)
		if err != nil {
			panic(fmt.Sprintf("Unable to connection to database: %v\n", err))
		}
		db.SetMaxIdleConns(cfg.PGMaxIdleConn)
		db.SetMaxOpenConns(cfg.PGMAxOpenConn)
	}
	return db
}
