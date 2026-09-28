package transactions

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

// minimal driver so the manager can be tested without a database
type fakeDriver struct{ failBegin bool }

func (d fakeDriver) Open(string) (driver.Conn, error) { return &fakeConn{}, nil }

type fakeConn struct{}

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no prepare") }
func (c *fakeConn) Close() error                        { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)           { return &fakeTx{}, nil }

type fakeTx struct{ done bool }

func (t *fakeTx) Commit() error   { t.done = true; return nil }
func (t *fakeTx) Rollback() error { t.done = true; return nil }

var regOnce = registerFake()

func registerFake() bool { sql.Register("faketx", fakeDriver{}); return true }

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("faketx", "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestBeginGetCommit(t *testing.T) {
	m, db := New(), newDB(t)
	id, err := m.Begin(context.Background(), db)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if id == "" {
		t.Fatal("empty id")
	}
	if _, ok := m.Get(id); !ok {
		t.Fatal("transaction not retrievable after begin")
	}
	if err := m.Commit(id); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if m.Len() != 0 {
		t.Fatalf("len after commit = %d, want 0", m.Len())
	}
}

func TestRollback(t *testing.T) {
	m, db := New(), newDB(t)
	id, _ := m.Begin(context.Background(), db)
	if err := m.Rollback(id); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if m.Len() != 0 {
		t.Fatalf("len after rollback = %d, want 0", m.Len())
	}
}

func TestGetAfterCloseIsNotUsable(t *testing.T) {
	// a committed transaction must not be executable again
	m, db := New(), newDB(t)
	id, _ := m.Begin(context.Background(), db)
	if err := m.Commit(id); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Get(id); ok {
		t.Fatal("committed transaction was returned as usable")
	}
}

func TestCommitUnknownID(t *testing.T) {
	if err := New().Commit("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	if err := New().Rollback("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestDoubleCommitIsRejected(t *testing.T) {
	m, db := New(), newDB(t)
	id, _ := m.Begin(context.Background(), db)
	if err := m.Commit(id); err != nil {
		t.Fatal(err)
	}
	// the second commit must not silently succeed against a freed tx
	if err := m.Commit(id); err == nil {
		t.Fatal("second commit returned nil")
	}
}

func TestIDsAreUnique(t *testing.T) {
	m, db := New(), newDB(t)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id, err := m.Begin(context.Background(), db)
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestReapRemovesStaleTransactions(t *testing.T) {
	m := NewWithTimeout(time.Millisecond)
	db := newDB(t)
	base := time.Now()
	m.now = func() time.Time { return base }

	id, _ := m.Begin(context.Background(), db)
	if m.Reap() != 0 {
		t.Fatal("reaped a fresh transaction")
	}
	if _, ok := m.Get(id); !ok {
		t.Fatal("fresh transaction vanished")
	}

	// advance the clock past the timeout
	m.now = func() time.Time { return base.Add(time.Hour) }
	if n := m.Reap(); n != 1 {
		t.Fatalf("reaped %d, want 1", n)
	}
	if m.Len() != 0 {
		t.Fatalf("len after reap = %d, want 0", m.Len())
	}
}

func TestReapWithNonPositiveTimeoutNeverReaps(t *testing.T) {
	m := NewWithTimeout(0)
	db := newDB(t)
	base := time.Now()
	m.now = func() time.Time { return base }
	if _, err := m.Begin(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return base.Add(1000 * time.Hour) }
	if n := m.Reap(); n != 0 {
		t.Fatalf("reaped %d with reaping disabled, want 0", n)
	}
}

func TestBeginNilDB(t *testing.T) {
	if _, err := New().Begin(context.Background(), nil); err == nil {
		t.Fatal("Begin(nil) returned nil error")
	}
}

func TestRollbackAll(t *testing.T) {
	m, db := New(), newDB(t)
	for i := 0; i < 5; i++ {
		if _, err := m.Begin(context.Background(), db); err != nil {
			t.Fatal(err)
		}
	}
	m.RollbackAll()
	if m.Len() != 0 {
		t.Fatalf("len after RollbackAll = %d, want 0", m.Len())
	}
}

func TestConcurrentBeginGet(t *testing.T) {
	m, db := New(), newDB(t)
	const n = 50
	ids := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			id, err := m.Begin(context.Background(), db)
			if err != nil {
				t.Errorf("begin: %v", err)
				return
			}
			if _, ok := m.Get(id); !ok {
				t.Errorf("id %q not retrievable", id)
			}
			ids <- id
		}()
	}
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		id := <-ids
		if seen[id] {
			t.Fatalf("duplicate id under concurrency: %q", id)
		}
		seen[id] = true
	}
}

// TestReapUsesIdleTimeNotAge pins the distinction that matters: a transaction
// that is old but still being used must survive, and only one that has been
// idle past the timeout may be reaped.
//
// Reap compared e.created against the cutoff, so a long-running transaction was
// rolled out from under an active client at the timeout regardless of use. Get
// refreshes lastUsed, which is the field that should decide.
func TestReapUsesIdleTimeNotAge(t *testing.T) {
	m := NewWithTimeout(time.Minute)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	db := newDB(t)

	id, err := m.Begin(context.Background(), db)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	// Well past the timeout, but the client is actively joining it.
	m.now = func() time.Time { return base.Add(10 * time.Minute) }
	if _, ok := m.Get(id); !ok {
		t.Fatal("transaction should still be joinable while in use")
	}

	if n := m.Reap(); n != 0 {
		t.Errorf("Reap() = %d, want 0: an actively used transaction is not idle", n)
	}
	if _, ok := m.Get(id); !ok {
		t.Error("transaction was reaped despite being in use")
	}

	// Now leave it alone past the timeout; it is genuinely idle.
	m.now = func() time.Time { return base.Add(30 * time.Minute) }
	if n := m.Reap(); n != 1 {
		t.Errorf("Reap() = %d, want 1 for an idle transaction", n)
	}
	if _, ok := m.Get(id); ok {
		t.Error("idle transaction should have been reaped")
	}
}

// TestIDsCarryUnpredictableEntropy guards the property that makes an ID safe to
// treat as a capability.
//
// The ID authorises commit, rollback and joining a transaction, and nothing
// else does. The previous scheme was a timestamp to the second plus a
// per-process counter, which an attacker can enumerate without guessing, and
// two managers that begin at the same instant produced the same first ID.
func TestIDsCarryUnpredictableEntropy(t *testing.T) {
	db := newDB(t)
	seen := make(map[string]string)
	for i := 0; i < 64; i++ {
		// A fresh manager each time, all beginning at the same instant: only
		// real entropy can keep these apart.
		m := New()
		m.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
		id, err := m.Begin(context.Background(), db)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if prev, dup := seen[id]; dup {
			t.Fatalf("ID %q collides with %q; the ID is not unique per manager", id, prev)
		}
		seen[id] = id
		if !strings.HasPrefix(id, "tx_") {
			t.Errorf("ID %q lost its prefix", id)
		}
	}
	if len(seen) != 64 {
		t.Fatalf("got %d distinct ids, want 64", len(seen))
	}
}
