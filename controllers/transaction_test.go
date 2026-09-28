package controllers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/transactions"
	"github.com/stretchr/testify/require"
)

// stubBeginner mimics the adapter by calling database/sql's real BeginTx.
//
// The obvious stub, returning a pre-made *sql.Tx, cannot catch the context
// lifetime bug: database/sql rolls a transaction back when the context passed
// to BeginTx is done, so the request that opened the transaction cancels it the
// moment the handler returns. Going through a real *sql.DB means the test
// exercises that cancellation for real instead of assuming it away.
type stubBeginner struct {
	db   *sql.DB
	tx   *sql.Tx
	err  error
	hits int
}

func (s *stubBeginner) GetTransactionCtx(ctx context.Context) (*sql.Tx, error) {
	s.hits++
	if s.err != nil {
		return nil, s.err
	}
	if s.db != nil {
		return s.db.BeginTx(ctx, nil)
	}
	return s.tx, nil
}

type stubDriver struct{}

func (stubDriver) Open(string) (driver.Conn, error) { return &stubConn{}, nil }

type stubConn struct{}

func (*stubConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no prepare") }
func (*stubConn) Close() error                        { return nil }
func (*stubConn) Begin() (driver.Tx, error)           { return &stubTx{}, nil }

type stubTx struct{}

func (*stubTx) Commit() error   { return nil }
func (*stubTx) Rollback() error { return nil }

var stubOnce = registerStub()

func registerStub() bool { sql.Register("presttxstub", stubDriver{}); return true }

func newStubDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("presttxstub", "")
	if err != nil {
		t.Fatalf("open stub db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestBeginReturnsTransactionID(t *testing.T) {
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{tx: mustTx(t, db)})

	rec := httptest.NewRecorder()
	h.Begin(rec, httptest.NewRequest(http.MethodPost, "/_tx", nil))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	id, _ := body["transaction_id"].(string)
	if id == "" {
		t.Fatalf("no transaction_id in %v", body)
	}
	if body["status"] != "open" {
		t.Fatalf("status field = %v, want open", body["status"])
	}
}

func mustTx(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

func TestBeginPropagatesAdapterError(t *testing.T) {
	h := NewTransactionHandler(&stubBeginner{err: errors.New("no connection")})
	rec := httptest.NewRecorder()
	h.Begin(rec, httptest.NewRequest(http.MethodPost, "/_tx", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestCommitSucceedsThenSecondIsGone(t *testing.T) {
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{tx: mustTx(t, db)})

	begin := httptest.NewRecorder()
	h.Begin(begin, httptest.NewRequest(http.MethodPost, "/_tx", nil))
	id := decode(t, begin)["transaction_id"].(string)

	commit := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/_tx/"+id, nil)
	req = mux.SetURLVars(req, map[string]string{"id": id})
	h.Commit(commit, req)
	if commit.Code != http.StatusOK {
		t.Fatalf("commit status = %d: %s", commit.Code, commit.Body)
	}
	if got := decode(t, commit)["status"]; got != "committed" {
		t.Fatalf("status = %v, want committed", got)
	}

	// a second commit must not report success
	again := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/_tx/"+id, nil)
	req2 = mux.SetURLVars(req2, map[string]string{"id": id})
	h.Commit(again, req2)
	if again.Code != http.StatusGone {
		t.Fatalf("second commit status = %d, want 410", again.Code)
	}
}

func TestRollbackReportsRolledBack(t *testing.T) {
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{tx: mustTx(t, db)})
	begin := httptest.NewRecorder()
	h.Begin(begin, httptest.NewRequest(http.MethodPost, "/_tx", nil))
	id := decode(t, begin)["transaction_id"].(string)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/_tx/"+id, nil)
	req = mux.SetURLVars(req, map[string]string{"id": id})
	h.Rollback(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if got := decode(t, rec)["status"]; got != "rolled back" {
		t.Fatalf("status = %v, want 'rolled back'", got)
	}
}

func TestUnknownTransactionIsGoneNotNotFound(t *testing.T) {
	// 410, because the caller is retrying with an ID that used to exist. A 404
	// would read as "this path is wrong" and invite a different fix.
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{tx: mustTx(t, db)})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/_tx/tx_nope", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "tx_nope"})
	h.Rollback(rec, req)
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", rec.Code)
	}
}

func TestLookupFindsOpenTransactionByHeader(t *testing.T) {
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{tx: mustTx(t, db)})
	begin := httptest.NewRecorder()
	h.Begin(begin, httptest.NewRequest(http.MethodPost, "/_tx", nil))
	id := decode(t, begin)["transaction_id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/db/schema/table", nil)
	if _, ok := h.Lookup(req); ok {
		t.Fatal("found a transaction without the header")
	}
	req.Header.Set("X-Prest-Transaction", id)
	if _, ok := h.Lookup(req); !ok {
		t.Fatal("did not find the open transaction by header")
	}
}

func TestLookupRejectsStaleHeader(t *testing.T) {
	// A header naming a committed transaction must not join it. Silently
	// ignoring the header and running outside the transaction would be worse:
	// the client believes its work is atomic and it is not.
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{tx: mustTx(t, db)})
	begin := httptest.NewRecorder()
	h.Begin(begin, httptest.NewRequest(http.MethodPost, "/_tx", nil))
	id := decode(t, begin)["transaction_id"].(string)
	c := httptest.NewRecorder()
	cq := httptest.NewRequest(http.MethodPost, "/_tx/"+id, nil)
	cq = mux.SetURLVars(cq, map[string]string{"id": id})
	h.Commit(c, cq)

	req := httptest.NewRequest(http.MethodGet, "/db/schema/table", nil)
	req.Header.Set("X-Prest-Transaction", id)
	if _, ok := h.Lookup(req); ok {
		t.Fatal("a committed transaction was joinable")
	}
}

func TestHandlerRejectsUnsupportedMethod(t *testing.T) {
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{tx: mustTx(t, db)})
	rec := httptest.NewRecorder()
	h.Handler()(rec, httptest.NewRequest(http.MethodGet, "/_tx", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestHandlerRoutesPostAndDelete(t *testing.T) {
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{tx: mustTx(t, db)})

	rec := httptest.NewRecorder()
	h.Handler()(rec, httptest.NewRequest(http.MethodPost, "/_tx", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST routed to %d, want 201", rec.Code)
	}
	id := decode(t, rec)["transaction_id"].(string)

	del := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/_tx/"+id, nil)
	req = mux.SetURLVars(req, map[string]string{"id": id})
	h.Handler()(del, req)
	if del.Code != http.StatusOK {
		t.Fatalf("DELETE routed to %d, want 200", del.Code)
	}
}

// The reaper must not fire on a transaction that is merely old but still in
// use within a request; this pins that a long-lived transaction is only reaped
// after the configured idle window.
func TestOpenTransactionSurvivesShortReapWindow(t *testing.T) {
	db := newStubDB(t)
	m := transactions.NewWithTimeout(time.Hour)
	beginner := &stubBeginner{tx: mustTx(t, db)}
	id, err := m.BeginWith(context.Background(), beginner)
	if err != nil {
		t.Fatal(err)
	}
	if n := m.Reap(); n != 0 {
		t.Fatalf("reaped %d within the window, want 0", n)
	}
	if _, ok := m.Get(id); !ok {
		t.Fatal("transaction disappeared inside its own window")
	}
}

// TestTransactionSurvivesTheRequestThatOpenedIt is the regression test for the
// context lifetime bug.
//
// database/sql rolls a transaction back when the context handed to BeginTx is
// done. The request that opens a transaction is finished before the client
// receives the ID, so passing r.Context() to BeginTx killed the transaction
// before it could be used: every commit and rollback returned 410, and every
// CRUD write silently fell outside the transaction.
//
// The stub goes through a real *sql.DB, so the rollback-on-cancel behaviour is
// the real one rather than something this test has to simulate.
func TestTransactionSurvivesTheRequestThatOpenedIt(t *testing.T) {
	db := newStubDB(t)
	h := NewTransactionHandler(&stubBeginner{db: db})

	// Open, exactly as the endpoint does, with a request that then completes.
	req := httptest.NewRequest(http.MethodPost, "/_transactions", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Begin(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	id, _ := decode(t, rec)["transaction_id"].(string)
	require.NotEmpty(t, id)

	// The opening request is now over. In a real server its context is done;
	// cancel() reproduces that exactly.
	cancel()

	// The transaction must still be open and committable.
	// Through the real router, so the path variable is populated the way it is
	// in production. A direct handler call would leave mux.Vars empty and 410
	// for an unrelated reason, which is exactly how this bug stayed hidden.
	rt := mux.NewRouter()
	rt.HandleFunc("/_transactions/{id}/commit", h.Commit).Methods(http.MethodPost)
	cRec := httptest.NewRecorder()
	rt.ServeHTTP(cRec, httptest.NewRequest(http.MethodPost, "/_transactions/"+id+"/commit", nil))
	require.Equal(t, http.StatusOK, cRec.Code,
		"transaction was rolled back when the request that opened it finished")
}
