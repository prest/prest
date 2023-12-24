package postgres

import (
	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/adapters/postgres/internal/connection"
)

// GetURI postgres connection URI
func (a Adapter) GetURI(DBName string) string {
	return a.conn.GetURI(DBName)
}

// Get get postgres connection
func (a Adapter) Get() (*sqlx.DB, error) {
	return a.conn.Get()
}

// GetPool of connection
func (a Adapter) GetPool() *connection.Pool {
	return a.conn
}

// AddDatabaseToPool add connection to pool
func (a Adapter) AddDatabaseToPool(name string, DB *sqlx.DB) {
	a.conn.AddDatabaseToPool(name, DB)
}

// MustGet get postgres connection
func (a Adapter) MustGet() *sqlx.DB {
	return a.conn.MustGet()
}

// SetDatabase set current database in use
// todo: remove when ctx is fully implemented
func (a Adapter) SetDatabase(name string) {
	a.conn.SetDatabase(name)
}

// GetDatabase get current database in use
func (a Adapter) GetDatabase() string {
	return a.conn.GetDatabase()
}
