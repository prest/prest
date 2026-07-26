package mock

import (
	"database/sql/driver"
)

// mockConn is the mock of driver.Conn
type mockConn struct{}

func (mc *mockConn) Begin() (driver.Tx, error)                    { return mc, nil }
func (mc *mockConn) Close() (err error)                           { return }
func (mc *mockConn) Prepare(q string) (st driver.Stmt, err error) { return }
func (mc *mockConn) Commit() (err error)                          { return }
func (mc *mockConn) Rollback() (err error)                        { return }
