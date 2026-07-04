package connection

import "github.com/jmoiron/sqlx"

// InjectDBForTest registers a mock or test *sqlx.DB in the connection pool.
// See .github/copilot-instructions.md (Postgres adapter unit tests) for conventions.
func (m *Manager) InjectDBForTest(uri string, db *sqlx.DB) {
	p := m.getPool()
	p.Mtx.Lock()
	p.DB[uri] = db
	p.Mtx.Unlock()
}

// ResetPoolForTest clears the connection pool and current database selection.
func (m *Manager) ResetPoolForTest() {
	p := m.getPool()
	p.Mtx.Lock()
	p.DB = make(map[string]*sqlx.DB)
	p.Mtx.Unlock()
	m.mu.Lock()
	m.currDatabase = ""
	m.mu.Unlock()
}

// SetDBConnectForTest replaces sqlx.Connect for unit tests and returns a restore function.
// See .github/copilot-instructions.md (Postgres adapter unit tests) for conventions.
func SetDBConnectForTest(fn func(driverName, dataSourceName string) (*sqlx.DB, error)) func() {
	orig := dbConnect
	dbConnect = fn
	return func() { dbConnect = orig }
}
