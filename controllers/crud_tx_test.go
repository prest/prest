package controllers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mockgen"

	"github.com/stretchr/testify/require"
)

// recordingTxExecutor is the transaction-aware write path. A test that only
// asserts a 201 would pass even if the transaction were ignored, so these tests
// assert this stub was reached.
//
// gomock supplies the other half of the proof: the non-transactional executor
// has no InsertCtx expectation in the transaction tests, so a single call to
// it fails the test as an unexpected call.
type recordingTxExecutor struct {
	scanner adapters.Scanner
	inserts int
	updates int
	deletes int
}

func (r *recordingTxExecutor) InsertWithTransaction(*sql.Tx, string, ...interface{}) adapters.Scanner {
	r.inserts++
	return r.scanner
}

func (r *recordingTxExecutor) UpdateWithTransaction(*sql.Tx, string, ...interface{}) adapters.Scanner {
	r.updates++
	return r.scanner
}

func (r *recordingTxExecutor) DeleteWithTransaction(*sql.Tx, string, ...interface{}) adapters.Scanner {
	r.deletes++
	return r.scanner
}

// txTestHandler builds a CRUDHandler whose only wired write path is the
// transaction one, plus the id of a genuinely open transaction.
func txTestHandler(t *testing.T, ctrl *gomock.Controller) (*CRUDHandler, *recordingTxExecutor, string) {
	t.Helper()

	db := newStubDB(t)
	tx, err := db.Begin()
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	beginner := &stubBeginner{tx: tx}
	// NewTransactionHandler owns the manager, so open the transaction through
	// the handler's own Lookup path to get an id that genuinely exists.
	h0 := NewTransactionHandler(beginner)
	id, err := h0.mgr.BeginWith(context.Background(), beginner)
	require.NoError(t, err)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).
		Return(`"name"`, "$1", []interface{}{"prest"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().
		InsertSQL("prest-test", "public", "test", `"name"`, "$1").
		Return(`INSERT INTO test`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"id":1}`))

	rec := &recordingTxExecutor{scanner: scanner}
	h := NewCRUDHandler(Deps{
		Builder:   builder,
		SQL:       sqlBuilder,
		Executor:  mockgen.NewMockQueryExecutor(ctrl), // no InsertCtx expected
		DB:        mockDatabaseRegistry(ctrl),
		TxExec:    rec,
		TxHandler: h0,
	})
	return h, rec, id
}

func insertReq(header, value string) *http.Request {
	r := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	if header != "" {
		r.Header.Set(transactionHeader, value)
	}
	return r
}

func TestCRUDInsert_JoinsOpenTransaction(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, rec, id := txTestHandler(t, ctrl)

	w := httptest.NewRecorder()
	h.Insert(w, insertReq(transactionHeader, id))

	require.Equal(t, http.StatusCreated, w.Code)
	require.Equal(t, 1, rec.inserts, "the write must go through InsertWithTransaction")
}

func TestCRUDInsert_NoHeaderUsesNonTransactionalPath(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).
		Return(`"name"`, "$1", []interface{}{"prest"}, nil)
	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().
		InsertSQL("prest-test", "public", "test", `"name"`, "$1").
		Return(`INSERT INTO test`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"id":1}`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().InsertCtx(gomock.Any(), `INSERT INTO test`, "prest").Return(scanner)

	rec := &recordingTxExecutor{}
	h := NewCRUDHandler(Deps{
		Builder:   builder,
		SQL:       sqlBuilder,
		Executor:  executor,
		DB:        mockDatabaseRegistry(ctrl),
		TxExec:    rec,
		TxHandler: NewTransactionHandler(&stubBeginner{}),
	})

	w := httptest.NewRecorder()
	h.Insert(w, insertReq("", ""))

	require.Equal(t, http.StatusCreated, w.Code)
	require.Zero(t, rec.inserts, "a request with no header must not touch the transaction path")
}

func TestCRUDStaleTransactionHeaderRunsNothing(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// A manager with nothing open. The header names an id that does not exist,
	// so the request must be refused and the statement must not run.
	rec := &recordingTxExecutor{}
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).
		Return(`"name"`, "$1", []interface{}{"prest"}, nil)
	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().
		InsertSQL("prest-test", "public", "test", `"name"`, "$1").
		Return(`INSERT INTO test`)

	h := NewCRUDHandler(Deps{
		Builder:   builder,
		SQL:       sqlBuilder,
		Executor:  mockgen.NewMockQueryExecutor(ctrl), // any InsertCtx call fails
		DB:        mockDatabaseRegistry(ctrl),
		TxExec:    rec,
		TxHandler: NewTransactionHandler(&stubBeginner{}),
	})

	w := httptest.NewRecorder()
	h.Insert(w, insertReq(transactionHeader, "tx_never_opened"))

	require.Equal(t, http.StatusGone, w.Code)
	require.Zero(t, rec.inserts, "a stale header must not fall through to a transactional write")
}

func TestCRUDTransactionHeaderWithoutAdapterSupport(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// TxExec nil: the adapter does not implement TxExecutor.
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).
		Return(`"name"`, "$1", []interface{}{"prest"}, nil)
	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().
		InsertSQL("prest-test", "public", "test", `"name"`, "$1").
		Return(`INSERT INTO test`)

	h := NewCRUDHandler(Deps{
		Builder:   builder,
		SQL:       sqlBuilder,
		Executor:  mockgen.NewMockQueryExecutor(ctrl),
		DB:        mockDatabaseRegistry(ctrl),
		TxHandler: NewTransactionHandler(&stubBeginner{}),
	})

	w := httptest.NewRecorder()
	h.Insert(w, insertReq(transactionHeader, "tx_anything"))

	require.Equal(t, http.StatusNotImplemented, w.Code,
		"a header on an adapter without transaction support must be refused, not ignored")
}

func TestCRUDBatchInsert_RefusesTransactionHeader(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// The adapter has no transaction-aware batch insert. The request must be
	// refused, because falling through to BatchInsertValuesCtx would be a write
	// outside the transaction the client asked for -- and silently so.
	db := mockDatabaseRegistry(ctrl)
	db.EXPECT().Aliases().Return([]string{"prest-test"}).AnyTimes()
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{
		// No Builder and no Executor: reaching either would panic, which is the
		// point -- the request must not get that far.
		DB:        db,
		TxExec:    &recordingTxExecutor{},
		TxHandler: NewTransactionHandler(&stubBeginner{}),
	})

	w := httptest.NewRecorder()
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	req.Header.Set(transactionHeader, "tx_anything")
	h.BatchInsert(w, req)

	require.Equal(t, http.StatusNotImplemented, w.Code)
	require.Contains(t, w.Body.String(), "does not support transactions")
}
