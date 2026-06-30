package controllers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/controllers/auth"
	pctx "github.com/prest/prest/v2/context"
	"github.com/stretchr/testify/require"
)

func withTestTimeout(ctx context.Context) context.Context {
	return context.WithValue(ctx, pctx.HTTPTimeoutKey, 60) //nolint:staticcheck
}

func crudRequest(method, path string, vars map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req = mux.SetURLVars(req, vars)
	return req.WithContext(withTestTimeout(req.Context()))
}

type recordingCacher struct {
	key   string
	value string
}

func (c *recordingCacher) BuntSet(key, value string) {
	c.key = key
	c.value = value
}

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

	req := crudRequest(http.MethodGet, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
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

	req := crudRequest(http.MethodGet, "/prest-test/bad@schema/test", map[string]string{
		"database": "prest-test", "schema": "bad@schema", "table": "test",
	})
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

	req := crudRequest(http.MethodGet, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest")
}

func TestCRUDHandler_Select_WithCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM t`)

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
	scanner.EXPECT().Bytes().Return([]byte(`[{"name":"cached"}]`)).Times(2)

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	cacher := &recordingCacher{}
	h := NewCRUDHandler(Deps{
		Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db, Cache: cacher,
	})

	url := "/prest-test/public/test?foo=bar"
	req := crudRequest(http.MethodGet, url, map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, url, cacher.key)
	require.Contains(t, cacher.value, "cached")
}

func TestCRUDHandler_Select_RelationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "missing", "read", "").Return([]string{"id"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"id"}).Return(`"id"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"id"`, "prest-test", "public", "missing").Return(`SELECT "id" FROM t`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New(`pq: relation "public.missing" does not exist`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCRUDHandler_Select_WithUserContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "test", "read", "alice").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM t`)

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
	scanner.EXPECT().Bytes().Return([]byte(`[]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	req = req.WithContext(withTestTimeout(
		withUser(req.Context(), auth.User{Username: "alice"}),
	))
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCRUDHandler_Insert_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).Return(`"name"`, "$1", []interface{}{"prest"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().InsertSQL("prest-test", "public", "test", `"name"`, "$1").Return(`INSERT INTO test`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"id":1}`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().InsertCtx(gomock.Any(), `INSERT INTO test`, "prest").Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Contains(t, rec.Body.String(), `"id":1`)
}

func TestCRUDHandler_Insert_RelationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).Return(`"name"`, "$1", []interface{}{"x"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New(`pq: relation "public.missing" does not exist`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().InsertCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPost, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "relation does not exist")
}

func TestCRUDHandler_BatchInsert_Values(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`"name"`, "$1", []interface{}{"a", "b"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().InsertSQL("prest-test", "public", "test", `"name"`, "$1").Return(`INSERT INTO test`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"id":1}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().BatchInsertValuesCtx(gomock.Any(), `INSERT INTO test`, "a", "b").Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestCRUDHandler_BatchInsert_Copy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`name,age`, "", []interface{}{"a", 1}, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().BatchInsertCopyCtx(gomock.Any(), "prest-test", "public", "test", []string{"name", "age"}, "a", 1).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Builder: builder, SQL: mockgen.NewMockSQLBuilder(ctrl), Executor: executor, DB: db})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	req.Header.Set("Prest-Batch-Method", "copy")
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestCRUDHandler_Delete_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("id=$1", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return(`"id"`, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().DeleteSQL("prest-test", "public", "test").Return(`DELETE FROM test`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"id":1}`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().DeleteCtx(gomock.Any(), `DELETE FROM test WHERE id=$1 RETURNING "id"`, 1).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodDelete, "/prest-test/public/test?id=1", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCRUDHandler_Update_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"new"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("id=$2", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL("prest-test", "public", "test", `name=$1`).Return(`UPDATE test SET name=$1`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"id":1}`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().UpdateCtx(gomock.Any(), `UPDATE test SET name=$1 WHERE id=$2`, "new", 1).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPatch, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCRUDHandler_Update_RelationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"x"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("", nil, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`UPDATE t`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New(`pq: relation "public.missing" does not exist`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().UpdateCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPatch, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func withUser(ctx context.Context, user auth.User) context.Context {
	return context.WithValue(ctx, pctx.UserInfoKey, user)
}
