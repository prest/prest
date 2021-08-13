package postgres

import (
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/adapters/postgres/internal/connection"
)

// GetURI postgres connection URI
func GetURI(DBName string) string {
	return connection.GetURI(DBName)
}

// Get get postgres connection
func Get(database string) (*sqlx.DB, error) {
	return connection.Get(database)
}

// GetPool of connection
func GetPool() *connection.Pool {
	return connection.GetPool()
}

// AddDatabaseToPool add connection to pool
func AddDatabaseToPool(name string, DB *sqlx.DB) {
	connection.AddDatabaseToPool(name, DB)
}

// MustGet get postgres connection
func MustGet(database string) *sqlx.DB {
	return connection.MustGet(database)
}
