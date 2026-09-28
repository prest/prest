package transactions

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned when a transaction ID is unknown to this process.
//
// It is deliberately not a 404-shaped error at the transport layer: a caller
// that passes a stale ID after a restart should be told the transaction is gone
// rather than handed a fresh one that silently discards its uncommitted work.
var ErrNotFound = errors.New("transaction not found")

// ErrClosed is returned when an operation is attempted on a transaction that has
// already been committed or rolled back.
var ErrClosed = errors.New("transaction already closed")

// DefaultTimeout bounds how long a transaction may sit open before the manager
// reaps it. A transaction that is never committed holds a connection and locks
// rows, so an abandoned one is a resource leak with a long fuse.
const DefaultTimeout = 30 * time.Minute

// Manager owns the set of open transactions for a process.
//
// The zero value is not usable; call New. A Manager is safe for concurrent use.
//
// Transactions are held in memory, which is the same constraint the existing
// OpenTransactions map had. That is a deliberate limit rather than an oversight:
// the issue asked for the mechanism, and wiring this to a shared store is a
// separate piece of work that should not be smuggled in here. A single-node
// deployment gets correct behaviour; a multi-node one needs the store.
type Manager struct {
	mu      sync.Mutex
	txns    map[string]*managed
	timeout time.Duration
	now     func() time.Time

	// idSeq makes IDs unique within a process. The random part of the ID is
	// what makes it safe to treat as a capability, and the sequence is a
	// belt-and-braces guarantee against collision within a process.
	//
	// The ID is a capability. Commit, rollback and joining a transaction are
	// all authorised by knowing the ID and by nothing else, so a client that
	// can guess another client's ID can commit or roll back its work. A
	// timestamp-and-counter ID is guessable in both parts, so the entropy has
	// to come from crypto/rand.
	idSeq uint64
}

// managed is one open transaction and the bookkeeping the manager needs.
type managed struct {
	tx       *sql.Tx
	created  time.Time
	lastUsed time.Time
	closed   bool
}

// New creates a Manager with the default reap timeout.
func New() *Manager {
	return &Manager{
		txns:    make(map[string]*managed),
		timeout: DefaultTimeout,
		now:     time.Now,
	}
}

// NewWithTimeout creates a Manager that reaps transactions idle for longer than
// timeout. A non-positive timeout disables reaping, which is only sensible in
// tests.
func NewWithTimeout(timeout time.Duration) *Manager {
	m := New()
	m.timeout = timeout
	return m
}

// Begin opens a transaction and registers it under a new ID.
func (m *Manager) Begin(ctx context.Context, db *sql.DB) (string, error) {
	if db == nil {
		return "", errors.New("transactions: nil database handle")
	}
	now := m.now()
	// newID takes the lock itself; Begin must not hold it while calling, or the
	// two deadlock. Reading idSeq outside the lock is a data race under
	// concurrent Begin, which the -race test for that case catches.
	id := m.newID(now)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.txns[id] = &managed{tx: tx, created: now, lastUsed: now}
	m.mu.Unlock()
	return id, nil
}

// Beginner opens a transaction. *sql.DB satisfies it, and so does any adapter
// exposing GetTransactionCtx, which is how the HTTP layer opens one without
// knowing the concrete adapter type.
type Beginner interface {
	GetTransactionCtx(ctx context.Context) (tx *sql.Tx, err error)
}

// BeginWith opens a transaction through a Beginner and registers it.
func (m *Manager) BeginWith(ctx context.Context, b Beginner) (string, error) {
	if b == nil {
		return "", errors.New("transactions: nil transaction beginner")
	}
	tx, err := b.GetTransactionCtx(ctx)
	if err != nil {
		return "", err
	}
	now := m.now()
	id := m.newID(now)
	m.mu.Lock()
	m.txns[id] = &managed{tx: tx, created: now, lastUsed: now}
	m.mu.Unlock()
	return id, nil
}

// Get returns the transaction registered under id.
//
// The bool reports whether the transaction is still usable. It is false both for
// an unknown ID and for a closed one, so callers cannot accidentally execute
// against a transaction that has already been committed.
func (m *Manager) Get(id string) (*sql.Tx, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.txns[id]
	if !ok || e.closed {
		return nil, false
	}
	e.lastUsed = m.now()
	return e.tx, true
}

// Commit commits the transaction and removes it from the manager.
func (m *Manager) Commit(id string) error {
	return m.finish(id, true)
}

// Rollback rolls the transaction back and removes it from the manager.
func (m *Manager) Rollback(id string) error {
	return m.finish(id, false)
}

func (m *Manager) finish(id string, commit bool) error {
	m.mu.Lock()
	e, ok := m.txns[id]
	if ok {
		delete(m.txns, id)
	}
	m.mu.Unlock()

	if !ok {
		return ErrNotFound
	}
	if e.closed {
		return ErrClosed
	}
	e.closed = true
	if commit {
		return e.tx.Commit()
	}
	return e.tx.Rollback()
}

// Len reports how many transactions are currently open.
func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.txns)
}

// Reap rolls back and forgets every transaction that has been open longer than
// the configured timeout. It returns the number reaped.
//
// Callers on a timer get the same guarantee as the automatic reaper without
// having to trust that it ran. Reap is safe to call concurrently.
func (m *Manager) Reap() int {
	// A non-positive timeout disables reaping. Without this guard a caller that
	// passed 0 to mean "never" would get a cutoff in the future, and every
	// transaction would look stale immediately.
	if m.timeout <= 0 {
		return 0
	}
	m.mu.Lock()
	cutoff := m.now().Add(-m.timeout)
	var stale []*managed
	for id, e := range m.txns {
		// lastUsed, not created: a transaction in active use must not be
		// reaped just because it is old. Get refreshes it on every join.
		if e.lastUsed.Before(cutoff) {
			stale = append(stale, e)
			delete(m.txns, id)
		}
	}
	m.mu.Unlock()

	for _, e := range stale {
		if !e.closed {
			e.closed = true
			_ = e.tx.Rollback()
		}
	}
	return len(stale)
}

// RollbackAll rolls back everything. Intended for shutdown.
// Start reaps idle transactions on a ticker and rolls everything back when ctx
// is done. It blocks until ctx is cancelled, so callers run it in its own
// goroutine.
//
// Without this, Reap and RollbackAll are never reached in production: an
// abandoned transaction keeps its connection and its row locks until the
// process exits, and a client looping on POST /_transactions can exhaust the
// connection pool. A non-positive timeout disables the ticker entirely, which
// is only sensible in tests.
func (m *Manager) Start(ctx context.Context) {
	if m.timeout <= 0 {
		<-ctx.Done()
		m.RollbackAll()
		return
	}
	interval := m.timeout / 2
	if interval < time.Second {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.RollbackAll()
			return
		case <-t.C:
			m.Reap()
		}
	}
}

func (m *Manager) RollbackAll() {
	m.mu.Lock()
	all := m.txns
	m.txns = make(map[string]*managed)
	m.mu.Unlock()
	for _, e := range all {
		if !e.closed {
			e.closed = true
			_ = e.tx.Rollback()
		}
	}
}

// newID returns a process-unique transaction ID. It takes the lock, so callers
// must not already hold it.
func (m *Manager) newID(now time.Time) string {
	m.mu.Lock()
	m.idSeq++
	seq := m.idSeq
	m.mu.Unlock()
	return "tx_" + randomToken() + "_" + itoa(seq)
}

// randomToken returns 128 bits of hex from crypto/rand.
//
// The ID authorises commit, rollback and joining a transaction, so it is
// treated as a bearer capability and needs to be unguessable. crypto/rand can
// fail, and there is no safe way to continue with a weak ID, so a failure
// panics: it means the system is broken, and a predictable transaction ID is
// worse than an unavailable one.
func randomToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("transactions: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
