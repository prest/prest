package postgres

import (
	"github.com/prest/prest/v2/adapters/postgres/internal/connection"

	"github.com/jmoiron/sqlx"
)

// GetURI postgres connection URI
func (adapter *postgres) GetURI(DBName string) string {
	return adapter.conn.GetURI(DBName)
}

// Get get postgres connection
func (adapter *postgres) Get() (*sqlx.DB, error) {
	return adapter.conn.Get()
}

// GetPool of connection
func (adapter *postgres) GetPool() *connection.Pool {
	return adapter.conn.GetPool()
}

// AddDatabaseToPool add connection to pool
func (adapter *postgres) AddDatabaseToPool(name string) (*sqlx.DB, error) {
	return adapter.conn.AddDatabaseToPool(name)
}

// MustGet get postgres connection
func (adapter *postgres) MustGet() *sqlx.DB {
	return adapter.conn.MustGet()
}

// SetDatabase set current database in use
func (adapter *postgres) SetDatabase(name string) {
	adapter.conn.SetDatabase(name)
}

// ConnManager returns the underlying connection manager (for tests).
func (adapter *postgres) ConnManager() *connection.Manager {
	return adapter.conn
}
