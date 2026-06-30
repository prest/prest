package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters/mockgen"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func TestCRUDHandler_Select_PermissionDenied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "test", "read", "").Return([]string{}, nil)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{
		Perms:    perms,
		DB:       db,
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		SQL:      mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
	})

	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	req = mux.SetURLVars(req, map[string]string{"database": "prest-test", "schema": "public", "table": "test"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "permission")
}

func TestCRUDHandler_Select_InvalidPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{
		DB:       db,
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		SQL:      mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		Perms:    mockgen.NewMockPermissionsChecker(ctrl),
	})

	req := httptest.NewRequest(http.MethodGet, "/prest-test/bad@schema/test", nil)
	req = mux.SetURLVars(req, map[string]string{"database": "prest-test", "schema": "bad@schema", "table": "test"})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid identifier")
}

func TestCRUDHandler_Select_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM "prest-test"."public"."test"`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"name":"prest"}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{
		Perms:    perms,
		SQL:      sqlBuilder,
		Builder:  builder,
		Executor: executor,
		DB:       db,
	})

	req := httptest.NewRequest(http.MethodGet, "/prest-test/public/test", nil)
	req = mux.SetURLVars(req, map[string]string{"database": "prest-test", "schema": "public", "table": "test"})
	req = req.WithContext(withTestTimeout(req.Context()))
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest")
}

func withTestTimeout(ctx context.Context) context.Context {
	return context.WithValue(ctx, pctx.HTTPTimeoutKey, 60) //nolint:staticcheck
}
