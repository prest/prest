package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/queryguard"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// selectDeps wires the mocks a successful Select needs, leaving the executor to
// the caller so each test controls what the query returns.
func selectDeps(t *testing.T, ctrl *gomock.Controller) Deps {
	t.Helper()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "orders", "read", "").Return([]string{"id"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"id"}).Return(`"id"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"id"`, "prest-test", "public", "orders").
		Return(`SELECT "id" FROM "prest-test"."public"."orders"`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	return Deps{
		Perms:   perms,
		SQL:     sqlBuilder,
		Builder: builder,
		DB:      mockDatabaseRegistry(ctrl),
	}
}

func selectRequest() *http.Request {
	return crudRequest(http.MethodGet, "/prest-test/public/orders", map[string]string{
		"database": "prest-test", "schema": "public", "table": "orders",
	})
}

// A query refused by Query Guard answers 422 with the rule and the reason, so a
// client can tell a policy rejection from a malformed request.
func TestCRUDHandler_Select_QueryGuardRejection(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rejection := &queryguard.RejectionError{
		Rule:   queryguard.RuleSeqScan,
		Reason: "Sequential Scan detected on table 'orders'.",
	}
	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(rejection).AnyTimes()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	deps := selectDeps(t, ctrl)
	deps.TableExecutor = executor

	rec := httptest.NewRecorder()
	NewCRUDHandler(deps).Select(rec, selectRequest())

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var body guardRejection
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "query rejected by Query Guard", body.Error)
	require.Equal(t, "Sequential Scan detected on table 'orders'.", body.Reason)
	require.Equal(t, queryguard.RuleSeqScan, body.Rule)
}

// Database failures keep answering 400: only policy rejections map to 422.
func TestCRUDHandler_Select_NonGuardErrorStaysBadRequest(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("connection refused")).AnyTimes()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	deps := selectDeps(t, ctrl)
	deps.TableExecutor = executor

	rec := httptest.NewRecorder()
	NewCRUDHandler(deps).Select(rec, selectRequest())

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "connection refused")
}

// Table statements run through TableExecutor when the composition root set one.
func TestNewCRUDHandlerPrefersTableExecutor(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guarded := mockgen.NewMockQueryExecutor(ctrl)
	plain := mockgen.NewMockQueryExecutor(ctrl)

	h := NewCRUDHandler(Deps{Executor: plain, TableExecutor: guarded})
	require.Same(t, guarded, h.executor)
}

// Deps built by hand (tests, plugins) keep working without a TableExecutor.
func TestNewCRUDHandlerFallsBackToExecutor(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	plain := mockgen.NewMockQueryExecutor(ctrl)

	h := NewCRUDHandler(Deps{Executor: plain})
	require.Same(t, plain, h.executor)
}

// An agent calling select_table gets the rule and reason in the JSON-RPC error
// data, not only inside the wrapped message, so it can rewrite its query.
func TestMCPHandler_RPC_SelectTable_QueryGuardRejection(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().TablePermissions("prest-test", "public", "orders", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "orders", "read", "").Return([]string{"id"}, nil)

	// Column discovery runs on the plain executor: catalog reads are never guarded.
	showScanner := mockgen.NewMockScanner(ctrl)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "orders").Return(showScanner)

	// The row read runs on the guarded executor and is refused.
	rejection := &queryguard.RejectionError{
		Rule:   queryguard.RuleMaxCost,
		Reason: "Estimated cost 78000.50 exceeds the maximum allowed cost of 50000.00.",
	}
	rowScanner := mockgen.NewMockScanner(ctrl)
	rowScanner.EXPECT().Err().Return(rejection).AnyTimes()
	tableExecutor := mockgen.NewMockQueryExecutor(ctrl)
	tableExecutor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(rowScanner)

	h := NewMCPHandler(Deps{
		Executor:      executor,
		TableExecutor: tableExecutor,
		Perms:         perms,
		DB:            mockDatabaseRegistry(ctrl),
		PGDatabase:    "prest-test",
	})

	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
		`"params":{"name":"prest.select_table","arguments":` +
		`{"database":"prest-test","schema":"public","table":"orders","limit":5}}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var resp struct {
		Error struct {
			Code    int            `json:"code"`
			Message string         `json:"message"`
			Data    guardRejection `json:"data"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, http.StatusUnprocessableEntity, resp.Error.Code)
	require.Contains(t, resp.Error.Message, "select table failed")
	require.Equal(t, queryguard.ErrRejected.Error(), resp.Error.Data.Error)
	require.Equal(t, queryguard.RuleMaxCost, resp.Error.Data.Rule)
	require.Equal(t, rejection.Reason, resp.Error.Data.Reason)
}

// Errors that are not policy rejections keep the previous 400 and carry no data.
func TestMCPHandler_RPC_NonGuardErrorStaysBadRequest(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewMCPHandler(Deps{DB: mockDatabaseRegistry(ctrl), PGDatabase: "prest-test"})

	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"nope"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported method")
	require.NotContains(t, rec.Body.String(), `"data"`)
}

// The MCP select_table tool reads user tables on behalf of an agent, so it runs
// through the guarded executor while catalog discovery keeps the plain one.
func TestNewMCPHandlerSplitsExecutors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guarded := mockgen.NewMockQueryExecutor(ctrl)
	plain := mockgen.NewMockQueryExecutor(ctrl)

	h := NewMCPHandler(Deps{Executor: plain, TableExecutor: guarded})
	require.Same(t, guarded, h.tableExecutor)
	require.Same(t, plain, h.executor)

	fallback := NewMCPHandler(Deps{Executor: plain})
	require.Same(t, plain, fallback.tableExecutor)
}
