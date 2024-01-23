package postgres

import (
	"database/sql"

	"github.com/prest/prest/adapters/postgres/internal/connection"
)

// GetURI postgres connection URI
func (a adapter) GetConnURI(DBName string) string {
	return a.pool.GetURI(DBName)
}

// Get get postgres connection
func (a adapter) GetConn() (*sql.DB, error) {
	db, err := a.pool.Get()
	if err != nil {
		return nil, err
	}
	return db.DB, nil
}

// GetPool of connection
func (a adapter) GetConnPool() *connection.Pool {
	return a.pool
}

// AddDatabaseToPool add connection to pool
func (a adapter) AddDatabaseToConnPool(name string, DB *sql.DB) {
	a.pool.AddDatabaseToPool(name, DB)
}

// MustGet get postgres connection, will panic if connection fails
func (a adapter) MustGetConn() *sql.DB {
	return a.pool.MustGet().DB
}

// SetDatabase set current database in use
// todo: remove when ctx is fully implemented
func (a adapter) SetCurrentConnDatabase(name string) {
	a.pool.SetDatabase(name)
}

// GetDatabase get current database in use
func (a adapter) GetCurrentConnDatabase() string {
	return a.pool.GetDatabase()
}
