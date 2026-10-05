package connection

import "github.com/jmoiron/sqlx"

// InjectDBForTest registers a mock *sqlx.DB under this manager's DSN.
func (m *Manager) InjectDBForTest(db *sqlx.DB) error {
	dsn, err := m.DSN()
	if err != nil {
		return err
	}
	p := m.getPool()
	p.Mtx.Lock()
	p.DB[dsn] = db
	p.Mtx.Unlock()
	return nil
}

// SetDBConnectForTest replaces sqlx.Connect for unit tests and returns a restore function.
// Callers must not use t.Parallel().
func SetDBConnectForTest(fn func(driverName, dataSourceName string) (*sqlx.DB, error)) func() {
	orig := dbConnect
	dbConnect = fn
	return func() { dbConnect = orig }
}
