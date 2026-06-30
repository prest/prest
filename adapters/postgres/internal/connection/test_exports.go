package connection

import "github.com/jmoiron/sqlx"

// InjectDBForTest registers a mock or test *sqlx.DB in the connection pool.
func InjectDBForTest(uri string, db *sqlx.DB) {
	p := GetPool()
	p.Mtx.Lock()
	p.DB[uri] = db
	p.Mtx.Unlock()
}

// ResetPoolForTest clears the connection pool and current database selection.
func ResetPoolForTest() {
	p := GetPool()
	p.Mtx.Lock()
	p.DB = make(map[string]*sqlx.DB)
	p.Mtx.Unlock()
	currDatabase = ""
}
