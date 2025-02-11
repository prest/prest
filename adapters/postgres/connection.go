package postgres

import (
	"github.com/prest/prest/v2/adapters/postgres/internal/connection"

	"github.com/jmoiron/sqlx"
)

// GetURI postgres connection URI
func GetURI(DBName string) string {
	return connection.GetURI(DBName)
}

// Get get postgres connection
func Get() (*sqlx.DB, error) {
	return connection.Get()
}

// GetPool of connection
func GetPool() *connection.Pool {
	return connection.GetPool()
}

// AddDatabaseToPool add connection to pool
func AddDatabaseToPool(name string) (*sqlx.DB, error) {
	return connection.AddDatabaseToPool(name)
}

// MustGet get postgres connection
func MustGet() *sqlx.DB {
	return connection.MustGet()
}

// SetDatabase set current database in use
// todo: remove when ctx is fully implemented
func SetDatabase(name string) {
	connection.SetDatabase(name)
}

// GetDatabase get current database in use
func GetDatabase() string {
	return connection.GetDatabase()
}
