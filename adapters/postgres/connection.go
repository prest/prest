package postgres

import (
	"database/sql"

	"github.com/prest/prest/adapters/postgres/internal/connection"
)

// GetURI postgres connection URI
func (a Adapter) GetConnURI(DBName string) string {
	return a.conn.GetURI(DBName)
}

// Get get postgres connection
func (a Adapter) GetConn() (*sql.DB, error) {
	db, err := a.conn.Get()
	if err != nil {
		return nil, err
	}
	return db.DB, nil
}

// GetPool of connection
func (a Adapter) GetConnPool() *connection.Pool {
	return a.conn
}

// AddDatabaseToPool add connection to pool
func (a Adapter) AddDatabaseToConnPool(name string, DB *sql.DB) {
	a.conn.AddDatabaseToPool(name, DB)
}

// MustGet get postgres connection, will panic if connection fails
func (a Adapter) MustGetConn() *sql.DB {
	return a.conn.MustGet().DB
}

// SetDatabase set current database in use
// todo: remove when ctx is fully implemented
func (a Adapter) SetCurrentConnDatabase(name string) {
	a.conn.SetDatabase(name)
}

// GetDatabase get current database in use
func (a Adapter) GetCurrentConnDatabase() string {
	return a.conn.GetDatabase()
}
