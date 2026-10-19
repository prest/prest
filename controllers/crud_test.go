package controllers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/access"
	"github.com/prest/prest/v2/adapters/mockgen"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/middlewares"
	"github.com/stretchr/testify/require"
)

func withTestTimeout(ctx context.Context) context.Context {
	return context.WithValue(ctx, pctx.HTTPTimeoutKey, 60) //nolint:staticcheck
}

func mockDatabaseRegistry(ctrl *gomock.Controller) *mockgen.MockDatabaseRegistry {
	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().Aliases().Return([]string{"prest-test"}).AnyTimes()
	db.EXPECT().IsRegistered(gomock.Any()).Return(true).AnyTimes()
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()
	db.EXPECT().PhysicalName(gomock.Any()).DoAndReturn(func(alias string) string { return alias }).AnyTimes()
	return db
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
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{}, nil)

	db := mockDatabaseRegistry(ctrl)

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
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockDatabaseRegistry(ctrl)

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
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM "prest-test"."public"."test"`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"name":"prest"}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	db := mockDatabaseRegistry(ctrl)

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

func TestCRUDHandler_Select_UsesContextAdapter(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	selected := mockgen.NewMockAdapter(ctrl)
	selected.EXPECT().IsRegistered("prest-test").Return(true)
	selected.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)
	selected.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	selected.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM "prest-test"."public"."test"`)
	selected.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	selected.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	selected.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	selected.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	selected.EXPECT().GroupByClause(gomock.Any()).Return("")
	selected.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	selected.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	selected.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"name":"prest"}]`))
	selected.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{
		Perms:    mockgen.NewMockPermissionsChecker(ctrl),
		SQL:      mockgen.NewMockSQLBuilder(ctrl),
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		DB:       mockgen.NewMockDatabaseRegistry(ctrl),
	})

	req := crudRequest(http.MethodGet, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	req = req.WithContext(context.WithValue(req.Context(), pctx.AdapterKey, selected))
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest")
}

func TestCRUDHandler_Select_TimeBucketClauseError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM "prest-test"."public"."test"`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", errors.New("invalid time_bucket interval"))

	h := NewCRUDHandler(Deps{
		Perms:   perms,
		SQL:     sqlBuilder,
		Builder: builder,
		DB:      mockDatabaseRegistry(ctrl),
	})

	req := crudRequest(http.MethodGet, "/prest-test/public/test?_time_bucket=2h", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "could not perform TimeBucketClause")
	require.Contains(t, rec.Body.String(), "invalid time_bucket interval")
}

func TestCRUDHandler_Select_TimeBucketClauseSuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"*"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"*"}).Return(`*`, nil)
	sqlBuilder.EXPECT().SelectSQL(`*`, "prest-test", "public", "test").Return(`SELECT * FROM "prest-test"."public"."test"`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("GROUP BY status")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return(`GROUP BY time_bucket('1 hour', "time")`, nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"status":"ok"}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, sql string, _ ...interface{}) adapters.Scanner {
			require.NotContains(t, sql, "GROUP BY status GROUP BY")
			require.Contains(t, sql, `GROUP BY status, time_bucket('1 hour', "time")`)
			return scanner
		},
	)

	h := NewCRUDHandler(Deps{
		Perms:    perms,
		SQL:      sqlBuilder,
		Builder:  builder,
		Executor: executor,
		DB:       mockDatabaseRegistry(ctrl),
	})

	req := crudRequest(http.MethodGet, "/prest-test/public/test?_groupby=status&_time_bucket=1h", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "ok")
}

type groupByBinderBuilder struct {
	*mockgen.MockRequestQueryBuilder
	placeholderID int
}

func (b *groupByBinderBuilder) GroupByClauseValues(_ *http.Request, initialPlaceholderID int) (string, []any) {
	b.placeholderID = initialPlaceholderID
	return `GROUP BY "status" HAVING AVG("age") > $2`, []any{"o'brien"}
}

func TestCRUDHandler_Select_GroupByBinder(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM "prest-test"."public"."test"`)

	inner := mockgen.NewMockRequestQueryBuilder(ctrl)
	inner.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	inner.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	inner.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	inner.EXPECT().WhereByRequest(gomock.Any(), 1).Return("name=$1", []interface{}{"prest"}, nil)
	inner.EXPECT().GroupByClause(gomock.Any()).Times(0)
	inner.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	inner.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	inner.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	builder := &groupByBinderBuilder{MockRequestQueryBuilder: inner}

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"name":"prest"}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), "prest", "o'brien").DoAndReturn(
		func(_ context.Context, sql string, _ ...interface{}) adapters.Scanner {
			require.Contains(t, sql, `GROUP BY "status" HAVING AVG("age") > $2`)
			return scanner
		},
	)

	h := NewCRUDHandler(Deps{
		Perms:    perms,
		SQL:      sqlBuilder,
		Builder:  builder,
		Executor: executor,
		DB:       mockDatabaseRegistry(ctrl),
	})

	req := crudRequest(http.MethodGet, "/prest-test/public/test?name=$eq.prest", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 2, builder.placeholderID)
}

func TestCRUDHandler_Select_WithClauses(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM "prest-test"."public"."test"`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("DISTINCT ON (name)", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return([]string{" JOIN other ON other.id=test.id"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("name=$1", []interface{}{"prest"}, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("GROUP BY name")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("ORDER BY name DESC", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("LIMIT 10", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"name":"prest"}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), "prest").Return(scanner)

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public/test?name=$eq.prest", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest")
}

func TestCRUDHandler_Select_CountFirst(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM "prest-test"."public"."test"`)
	sqlBuilder.EXPECT().SelectSQL(`SELECT count(*) as count FROM "prest-test"."public"."test"`, "prest-test", "public", "test").Return(`SELECT count(*) as count FROM "prest-test"."public"."test"`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return(`SELECT count(*) as count FROM "prest-test"."public"."test"`, nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	countScanner := mockgen.NewMockScanner(ctrl)
	countScanner.EXPECT().Err().Return(nil)
	countScanner.EXPECT().Bytes().Return([]byte(`[{"count":1}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCountCtx(gomock.Any(), gomock.Any()).Return(countScanner)

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public/test?_count=*&_count_first=true", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"count":1`)
}

func TestCRUDHandler_Select_WithCache(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM t`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"name":"cached"}]`)).Times(2)

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	db := mockDatabaseRegistry(ctrl)

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
	require.Equal(t, middlewares.CacheKey(req), cacher.key)
	require.Contains(t, cacher.value, "cached")
}

func TestCRUDHandler_Select_RelationNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "missing", "read", "").Return([]string{"id"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"id"}).Return(`"id"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"id"`, "prest-test", "public", "missing").Return(`SELECT "id" FROM t`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New(`pq: relation "public.missing" does not exist`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCRUDHandler_Select_ErrRelationNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "missing", "read", "").Return([]string{"id"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"id"}).Return("`id`", nil)
	sqlBuilder.EXPECT().SelectSQL("`id`", "prest-test", "public", "missing").Return("SELECT `id` FROM t")

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(fmt.Errorf("missing: %w", adapters.ErrRelationNotFound))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCRUDHandler_Select_WithUserContext(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "alice").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM t`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	db := mockDatabaseRegistry(ctrl)

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
	t.Parallel()

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

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Contains(t, rec.Body.String(), `"id":1`)
}

func TestCRUDHandler_Insert_RelationNotFound(t *testing.T) {
	t.Parallel()

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

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPost, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "relation does not exist")
}

func TestCRUDHandler_Insert_InvalidPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{
		DB:       db,
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		SQL:      mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
	})

	req := crudRequest(http.MethodPost, "/prest-test/bad@schema/test", map[string]string{
		"database": "prest-test", "schema": "bad@schema", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid identifier")
}

func TestCRUDHandler_BatchInsert_Values(t *testing.T) {
	t.Parallel()

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

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestCRUDHandler_BatchInsert_Copy(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`name,age`, "", []interface{}{"a", 1}, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().BatchInsertCopyCtx(gomock.Any(), "prest-test", "public", "test", []string{"name", "age"}, "a", 1).Return(scanner)

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: mockgen.NewMockSQLBuilder(ctrl), Executor: executor, DB: db})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	req.Header.Set("Prest-Batch-Method", "copy")
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestCRUDHandler_Delete_Success(t *testing.T) {
	t.Parallel()

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

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodDelete, "/prest-test/public/test?id=1", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCRUDHandler_Delete_InvalidPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{
		DB:       db,
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		SQL:      mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
	})

	req := crudRequest(http.MethodDelete, "/prest-test/public/bad;table", map[string]string{
		"database": "prest-test", "schema": "public", "table": "bad;table",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid identifier")
}

func TestCRUDHandler_Update_Success(t *testing.T) {
	t.Parallel()

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

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPatch, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCRUDHandler_Update_RelationNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"x"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("id=$2", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`UPDATE t`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New(`pq: relation "public.missing" does not exist`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().UpdateCtx(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: db})
	req := crudRequest(http.MethodPatch, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCRUDHandler_Update_InvalidPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockDatabaseRegistry(ctrl)

	h := NewCRUDHandler(Deps{
		DB:       db,
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		SQL:      mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
	})

	req := crudRequest(http.MethodPatch, "/prest-test/bad@schema/test", map[string]string{
		"database": "prest-test", "schema": "bad@schema", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid identifier")
}

func TestCRUDHandler_Select_UnregisteredDB(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("invalid").Return(false)

	h := NewCRUDHandler(Deps{DB: db})

	req := crudRequest(http.MethodGet, "/invalid/public/test", map[string]string{
		"database": "invalid", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// Without [[databases]], a name outside pg.database and
// pg.allowed_databases is rejected with 400 before any query, so no pool is
// opened for it.
func TestCRUDHandler_Select_NoRegistry_UnknownDB_400(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("anything").Return(false)

	h := NewCRUDHandler(Deps{
		DB:       db,
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		Perms:    mockgen.NewMockPermissionsChecker(ctrl),
	})

	req := crudRequest(http.MethodGet, "/anything/public/test", map[string]string{
		"database": "anything", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "database not registered")
}

func TestCRUDHandler_Select_PermissionErrorOnFields(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return(nil, errors.New("permission denied"))

	h := NewCRUDHandler(Deps{
		Perms: perms,
		DB:    mockDatabaseRegistry(ctrl),
	})

	req := crudRequest(http.MethodGet, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCRUDHandler_Select_NoPermittedFields(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{}, nil)

	h := NewCRUDHandler(Deps{
		Perms: perms,
		DB:    mockDatabaseRegistry(ctrl),
	})

	req := crudRequest(http.MethodGet, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "don't have permission")
}

func TestCRUDHandler_Insert_UnregisteredDB(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("invalid").Return(false)

	h := NewCRUDHandler(Deps{DB: db})

	req := crudRequest(http.MethodPost, "/invalid/public/test", map[string]string{
		"database": "invalid", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCRUDHandler_Delete_UnregisteredDB(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("invalid").Return(false)

	h := NewCRUDHandler(Deps{DB: db})

	req := crudRequest(http.MethodDelete, "/invalid/public/test", map[string]string{
		"database": "invalid", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCRUDHandler_Update_UnregisteredDB(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("invalid").Return(false)

	h := NewCRUDHandler(Deps{DB: db})

	req := crudRequest(http.MethodPatch, "/invalid/public/test", map[string]string{
		"database": "invalid", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCRUDHandler_BatchInsert_UnregisteredDB(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("invalid").Return(false)

	h := NewCRUDHandler(Deps{DB: db})

	req := crudRequest(http.MethodPost, "/invalid/public/test", map[string]string{
		"database": "invalid", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func withUser(ctx context.Context, user auth.User) context.Context {
	return context.WithValue(ctx, pctx.UserInfoKey, user)
}

func baseSelectMocks(ctrl *gomock.Controller) (
	*mockgen.MockPermissionsChecker,
	*mockgen.MockSQLBuilder,
	*mockgen.MockRequestQueryBuilder,
	*mockgen.MockQueryExecutor,
	*mockgen.MockDatabaseRegistry,
) {
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return(`"name"`, nil)
	sqlBuilder.EXPECT().SelectSQL(`"name"`, "prest-test", "public", "test").Return(`SELECT "name" FROM t`)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	return perms, sqlBuilder, builder, executor, db
}

func expectSelectBuilderHappyPath(builder *mockgen.MockRequestQueryBuilder) {
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)
}

func runSelect(t *testing.T, h *CRUDHandler, method string) *httptest.ResponseRecorder {
	t.Helper()
	req := crudRequest(method, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Select(rec, req)
	return rec
}

func TestCRUDHandler_Select_SelectFieldsError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "read", "").Return([]string{"name"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().SelectFields([]string{"name"}).Return("", errors.New("invalid column"))

	h := NewCRUDHandler(Deps{
		Perms: perms,
		SQL:   sqlBuilder,
		DB:    mockDatabaseRegistry(ctrl),
	})

	rec := runSelect(t, h, http.MethodGet)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid column")
}

func TestCRUDHandler_Select_DistinctClauseError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", errors.New("bad distinct"))

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	rec := runSelect(t, h, http.MethodGet)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "Distinct")
}

func TestCRUDHandler_Select_CountByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", errors.New("bad count"))

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	rec := runSelect(t, h, http.MethodGet)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "CountByRequest")
}

func TestCRUDHandler_Select_JoinByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, errors.New("bad join"))

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	rec := runSelect(t, h, http.MethodGet)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "JoinByRequest")
}

func TestCRUDHandler_Select_WhereByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, errors.New("bad where"))

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	rec := runSelect(t, h, http.MethodGet)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "WhereByRequest")
}

func TestCRUDHandler_Select_OrderByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", errors.New("bad order"))

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	rec := runSelect(t, h, http.MethodGet)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "OrderByRequest")
}

func TestCRUDHandler_Select_PaginateError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().CountByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().JoinByRequest(gomock.Any()).Return(nil, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().GroupByClause(gomock.Any()).Return("")
	builder.EXPECT().TimeBucketClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", errors.New("bad page"))

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	rec := runSelect(t, h, http.MethodGet)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "PaginateIfPossible")
}

func TestCRUDHandler_Select_ExecutorError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	expectSelectBuilderHappyPath(builder)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("query failed"))
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	rec := runSelect(t, h, http.MethodGet)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "query failed")
}

func TestCRUDHandler_Select_NoCacheOnHead(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	expectSelectBuilderHappyPath(builder)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	cacher := &recordingCacher{}
	h := NewCRUDHandler(Deps{
		Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db, Cache: cacher,
	})
	rec := runSelect(t, h, http.MethodHead)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, cacher.key)
}

func TestCRUDHandler_Select_NonUserContextIgnored(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	expectSelectBuilderHappyPath(builder)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	req = req.WithContext(withTestTimeout(context.WithValue(req.Context(), pctx.UserInfoKey, "not-a-user")))
	rec := httptest.NewRecorder()
	h.Select(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCRUDHandler_Insert_ParseInsertError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).Return("", "", nil, errors.New("bad body"))

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl),
		Builder: builder,
		DB:      mockDatabaseRegistry(ctrl),
	})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "InsertInTables")
}

func TestCRUDHandler_Insert_ExecutorError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).Return(`"name"`, "$1", []interface{}{"x"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("duplicate key"))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().InsertCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "InsertInTables")
}

func TestCRUDHandler_BatchInsert_InvalidPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewCRUDHandler(Deps{DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPost, "/prest-test/bad@schema/test", map[string]string{
		"database": "prest-test", "schema": "bad@schema", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid identifier")
}

func TestCRUDHandler_BatchInsert_ParseError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return("", "", nil, errors.New("bad batch"))

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "BatchInsertInTables")
}

func TestCRUDHandler_BatchInsert_ValuesRelationNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`"name"`, "$1", []interface{}{"a"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New(`pq: relation "public.missing" does not exist`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().BatchInsertValuesCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPost, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "relation does not exist")
}

func TestCRUDHandler_BatchInsert_CopyRelationNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`name`, "", []interface{}{"a"}, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New(`pq: relation "public.missing" does not exist`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().BatchInsertCopyCtx(gomock.Any(), "prest-test", "public", "missing", []string{"name"}, "a").Return(scanner)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPost, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	req.Header.Set("Prest-Batch-Method", "copy")
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCRUDHandler_BatchInsert_ValuesExecutorError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`"name"`, "$1", []interface{}{"a"}, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("batch failed"))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().BatchInsertValuesCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "BatchInsertInTables")
}

func TestCRUDHandler_BatchInsert_CopyExecutorError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`name`, "", []interface{}{"a"}, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("copy failed"))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().BatchInsertCopyCtx(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPost, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	req.Header.Set("Prest-Batch-Method", "COPY")
	rec := httptest.NewRecorder()
	h.BatchInsert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCRUDHandler_Delete_WhereByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, errors.New("bad where"))

	h := NewCRUDHandler(Deps{Builder: builder, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodDelete, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "WhereByRequest")
}

func TestCRUDHandler_Delete_ReturningByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("id=$1", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", errors.New("bad returning"))

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().DeleteSQL("prest-test", "public", "test").Return(`DELETE FROM test`)

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodDelete, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "ReturningByRequest")
}

func TestCRUDHandler_Delete_RelationNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("id=$1", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().DeleteSQL("prest-test", "public", "missing").Return(`DELETE FROM missing`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New(`pq: relation "public.missing" does not exist`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().DeleteCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodDelete, "/prest-test/public/missing", map[string]string{
		"database": "prest-test", "schema": "public", "table": "missing",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "relation does not exist")
}

func TestCRUDHandler_Delete_ExecutorError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("id=$1", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().DeleteSQL(gomock.Any(), gomock.Any(), gomock.Any()).Return(`DELETE FROM test`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("delete failed"))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().DeleteCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodDelete, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "DeleteFromTable")
}

// An empty WHERE (no filter query params) must never reach the executor:
// a bare DELETE wipes the whole table (GHSA-r867-ghgv-mrp9).
func TestCRUDHandler_Delete_UnscopedRejected(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)

	h := NewCRUDHandler(Deps{
		Builder:  builder,
		SQL:      mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		DB:       mockDatabaseRegistry(ctrl),
	})
	req := crudRequest(http.MethodDelete, "/prest-test/public/test?_returning=*", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "refusing DELETE without a WHERE condition")
}

func TestCRUDHandler_Delete_UnscopedAllowedByConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().DeleteSQL("prest-test", "public", "test").Return(`DELETE FROM test`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().DeleteCtx(gomock.Any(), `DELETE FROM test`).Return(scanner)

	h := NewCRUDHandler(Deps{
		Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl),
		AllowUnscopedWrites: true,
	})
	req := crudRequest(http.MethodDelete, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

// PUT and PATCH share Update; an empty WHERE would rewrite every row.
func TestCRUDHandler_Update_UnscopedRejected(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			builder := mockgen.NewMockRequestQueryBuilder(ctrl)
			builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"x"}, nil)
			builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("", nil, nil)

			sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
			sqlBuilder.EXPECT().UpdateSQL("prest-test", "public", "test", `name=$1`).Return(`UPDATE test SET name=$1`)

			h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl),
				Builder:  builder,
				SQL:      sqlBuilder,
				Executor: mockgen.NewMockQueryExecutor(ctrl),
				DB:       mockDatabaseRegistry(ctrl),
			})
			req := crudRequest(method, "/prest-test/public/test", map[string]string{
				"database": "prest-test", "schema": "public", "table": "test",
			})
			rec := httptest.NewRecorder()
			h.Update(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), "refusing "+method+" without a WHERE condition")
		})
	}
}

func TestCRUDHandler_Update_UnscopedAllowedByConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"x"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("", nil, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL("prest-test", "public", "test", `name=$1`).Return(`UPDATE test SET name=$1`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"rows_affected":2}`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().UpdateCtx(gomock.Any(), `UPDATE test SET name=$1`, "x").Return(scanner)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl),
		Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl),
		AllowUnscopedWrites: true,
	})
	req := crudRequest(http.MethodPatch, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCRUDHandler_Update_SetByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return("", nil, errors.New("bad set"))

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPatch, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "UPDATE")
}

func TestCRUDHandler_Update_WhereByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"x"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("", nil, errors.New("bad where"))

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL("prest-test", "public", "test", `name=$1`).Return(`UPDATE test SET name=$1`)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPatch, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "WhereByRequest")
}

func TestCRUDHandler_Update_ReturningByRequestError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"x"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("id=$2", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", errors.New("bad returning"))

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL("prest-test", "public", "test", `name=$1`).Return(`UPDATE test SET name=$1`)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPatch, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "ReturningByRequest")
}

func TestCRUDHandler_Update_ExecutorError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"x"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("id=$2", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`UPDATE t`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("update failed"))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().UpdateCtx(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPatch, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "update failed")
}

func TestCRUDHandler_Update_WithReturning(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"new"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("id=$2", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return(`"id","name"`, nil)

	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL("prest-test", "public", "test", `name=$1`).Return(`UPDATE test SET name=$1`)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"id":1,"name":"new"}`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().UpdateCtx(gomock.Any(), `UPDATE test SET name=$1 WHERE id=$2 RETURNING "id","name"`, "new", 1).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
	req := crudRequest(http.MethodPatch, "/prest-test/public/test", map[string]string{
		"database": "prest-test", "schema": "public", "table": "test",
	})
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"new"`)
}

func TestNewCRUDHandler(t *testing.T) {
	t.Parallel()

	h := NewCRUDHandler(Deps{})
	require.NotNil(t, h)
}

// A refused connection answers 503 with a fixed message: the driver text
// ("dial tcp 10.0.0.5:5432: connect: connection refused") leaks the host.
func TestCRUDHandler_Select_DatabaseUnavailable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms, sqlBuilder, builder, executor, db := baseSelectMocks(ctrl)
	expectSelectBuilderHappyPath(builder)

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(dialError())
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Perms: perms, SQL: sqlBuilder, Builder: builder, Executor: executor, DB: db})
	rec := runSelect(t, h, http.MethodGet)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "database unavailable")
	require.NotContains(t, rec.Body.String(), "dial")
}

// Write paths map connection failures the same way as reads.
func TestCRUDHandler_Writes_DatabaseUnavailable(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"database": "prest-test", "schema": "public", "table": "test"}
	cases := []struct {
		name   string
		method string
		setup  func(*mockgen.MockRequestQueryBuilder, *mockgen.MockSQLBuilder, *mockgen.MockQueryExecutor, *mockgen.MockScanner)
		call   func(*CRUDHandler, http.ResponseWriter, *http.Request)
	}{
		{"insert", http.MethodPost, func(b *mockgen.MockRequestQueryBuilder, s *mockgen.MockSQLBuilder, e *mockgen.MockQueryExecutor, sc *mockgen.MockScanner) {
			b.EXPECT().ParseInsertRequest(gomock.Any()).Return(`"name"`, "$1", []interface{}{"x"}, nil)
			s.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)
			e.EXPECT().InsertCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(sc)
		}, (*CRUDHandler).Insert},
		{"batch insert", http.MethodPost, func(b *mockgen.MockRequestQueryBuilder, s *mockgen.MockSQLBuilder, e *mockgen.MockQueryExecutor, sc *mockgen.MockScanner) {
			b.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`"name"`, "($1)", []interface{}{"x"}, nil)
			s.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)
			e.EXPECT().BatchInsertValuesCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(sc)
		}, (*CRUDHandler).BatchInsert},
		{"delete", http.MethodDelete, func(b *mockgen.MockRequestQueryBuilder, s *mockgen.MockSQLBuilder, e *mockgen.MockQueryExecutor, sc *mockgen.MockScanner) {
			b.EXPECT().WhereByRequest(gomock.Any(), 1).Return("id=$1", []interface{}{1}, nil)
			b.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)
			s.EXPECT().DeleteSQL(gomock.Any(), gomock.Any(), gomock.Any()).Return(`DELETE`)
			e.EXPECT().DeleteCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(sc)
		}, (*CRUDHandler).Delete},
		{"update", http.MethodPut, func(b *mockgen.MockRequestQueryBuilder, s *mockgen.MockSQLBuilder, e *mockgen.MockQueryExecutor, sc *mockgen.MockScanner) {
			b.EXPECT().SetByRequest(gomock.Any(), 1).Return(`"name"=$1`, []interface{}{"x"}, nil)
			b.EXPECT().WhereByRequest(gomock.Any(), 2).Return("id=$2", []interface{}{1}, nil)
			b.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)
			s.EXPECT().UpdateSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`UPDATE`)
			e.EXPECT().UpdateCtx(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(sc)
		}, (*CRUDHandler).Update},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			builder := mockgen.NewMockRequestQueryBuilder(ctrl)
			sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
			executor := mockgen.NewMockQueryExecutor(ctrl)
			scanner := mockgen.NewMockScanner(ctrl)
			scanner.EXPECT().Err().Return(dialError())
			tc.setup(builder, sqlBuilder, executor, scanner)

			h := NewCRUDHandler(Deps{Perms: permsAllowAll(ctrl), Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl)})
			rec := httptest.NewRecorder()
			tc.call(h, rec, crudRequest(tc.method, "/prest-test/public/test", vars))

			require.Equal(t, http.StatusServiceUnavailable, rec.Code)
			require.Contains(t, rec.Body.String(), "database unavailable")
			require.NotContains(t, rec.Body.String(), "10.0.0.5")
		})
	}
}

// permsFor returns a PermissionsChecker with the given write and read field
// allowlists; readGranted is the table-level "read" permission.
func permsFor(ctrl *gomock.Controller, write, read []string, readGranted bool) *mockgen.MockPermissionsChecker {
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().
		FieldsPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "write", gomock.Any()).
		Return(write, nil).AnyTimes()
	perms.EXPECT().
		FieldsPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "read", gomock.Any()).
		Return(read, nil).AnyTimes()
	perms.EXPECT().
		TablePermissions(gomock.Any(), gomock.Any(), gomock.Any(), "read", gomock.Any()).
		Return(readGranted).AnyTimes()
	return perms
}

// permsAllowAll is an unrestricted PermissionsChecker for write tests that are
// not about field permissions.
func permsAllowAll(ctrl *gomock.Controller) *mockgen.MockPermissionsChecker {
	return permsFor(ctrl, []string{"*"}, []string{"*"}, true)
}

func crudRequestBody(method, path, body string, vars map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = mux.SetURLVars(req, vars)
	return req.WithContext(withTestTimeout(req.Context()))
}

var testTableVars = map[string]string{"database": "prest-test", "schema": "public", "table": "test"}

// strippedQuery matches a request whose query string was removed, proving the
// field allowlist cannot be steered by _select/_join on a write.
type strippedQuery struct{}

func (strippedQuery) Matches(x interface{}) bool {
	r, ok := x.(*http.Request)
	return ok && r.URL.RawQuery == ""
}

func (strippedQuery) String() string { return "request without query string" }

// GHSA-h25q-9rmj-646h: a body setting a column outside the write allowlist is
// rejected before any SQL is built (strict mocks: no builder call expected).
func TestCRUDHandler_Insert_RejectsUnwritableField(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	h := NewCRUDHandler(Deps{
		Builder: mockgen.NewMockRequestQueryBuilder(ctrl), SQL: mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl), DB: mockDatabaseRegistry(ctrl),
		Perms: permsFor(ctrl, []string{"id", "name"}, []string{"id", "name"}, true),
	})
	req := crudRequestBody(http.MethodPost, "/prest-test/public/test", `{"name":"x","role":"admin"}`, testTableVars)
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `column \"role\" is not writable`)
}

func TestCRUDHandler_Insert_AllowedFieldsReachBuilder(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	const body = `{"name":"x"}`
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).DoAndReturn(
		func(r *http.Request) (string, string, []interface{}, error) {
			// The gate consumed the body; the builder must still get every byte.
			got, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Equal(t, body, string(got))
			return `"name"`, "$1", []interface{}{"x"}, nil
		})
	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)
	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"id":1,"name":"x"}`))
	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().InsertCtx(gomock.Any(), `INSERT`, "x").Return(scanner)

	h := NewCRUDHandler(Deps{
		Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl),
		Perms: permsFor(ctrl, []string{"id", "name"}, []string{"id", "name"}, true),
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, crudRequestBody(http.MethodPost, "/prest-test/public/test", body, testTableVars))

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, `{"id":1,"name":"x"}`, rec.Body.String())
}

// insertReachesBuilder runs an Insert whose builder and executor succeed and
// returns the recorder, for tests about which requests pass the gate.
func insertReachesBuilder(t *testing.T, ctrl *gomock.Controller, perms adapters.PermissionsChecker, body, response string) *httptest.ResponseRecorder {
	t.Helper()
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().ParseInsertRequest(gomock.Any()).Return(`"role"`, "$1", []interface{}{"admin"}, nil)
	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)
	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(response))
	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().InsertCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl), Perms: perms})
	rec := httptest.NewRecorder()
	h.Insert(rec, crudRequestBody(http.MethodPost, "/prest-test/public/test", body, testTableVars))
	return rec
}

func TestCRUDHandler_Insert_WriteWildcardSkipsGate(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	rec := insertReachesBuilder(t, ctrl, permsAllowAll(ctrl), `{"role":"admin"}`, `{"id":1}`)
	require.Equal(t, http.StatusCreated, rec.Code)
}

// No fields configured on the write grant keeps writes unrestricted.
func TestCRUDHandler_Insert_EmptyWriteAllowlistUnrestricted(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	rec := insertReachesBuilder(t, ctrl, permsFor(ctrl, nil, []string{"*"}, true), `{"role":"admin"}`, `{"id":1}`)
	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestCRUDHandler_Insert_WriteAllowlistIgnoresQuery(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().
		FieldsPermissions(strippedQuery{}, "prest-test", "public", "test", "write", "ada").
		Return([]string{"name"}, nil)
	h := NewCRUDHandler(Deps{
		Builder: mockgen.NewMockRequestQueryBuilder(ctrl), SQL: mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl), DB: mockDatabaseRegistry(ctrl), Perms: perms,
	})
	req := crudRequestBody(http.MethodPost, "/prest-test/public/test?_select=role", `{"role":"admin"}`, testTableVars)
	req = req.WithContext(context.WithValue(req.Context(), pctx.UserInfoKey, auth.User{Username: "ada"}))
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCRUDHandler_Insert_GateRejections(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		body        string
		contains    string
	}{
		// The builder only decodes the first value, so the first value is what counts.
		{"first JSON value carries an unwritable key", `{"role":"a"} {"name":"x"}`, "not writable"},
		{"column match is case-sensitive", `{"Name":"x"}`, "not writable"},
		{"truncated body fails closed", `{`, "could not parse request body"},
		{"dotted key needs an exact allowlist entry", `{"name.x":"y"}`, "not writable"},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			h := NewCRUDHandler(Deps{
				Builder: mockgen.NewMockRequestQueryBuilder(ctrl), SQL: mockgen.NewMockSQLBuilder(ctrl),
				Executor: mockgen.NewMockQueryExecutor(ctrl), DB: mockDatabaseRegistry(ctrl),
				Perms: permsFor(ctrl, []string{"name"}, []string{"name"}, true),
			})
			rec := httptest.NewRecorder()
			h.Insert(rec, crudRequestBody(http.MethodPost, "/prest-test/public/test", tc.body, testTableVars))

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), tc.contains)
		})
	}
}

func TestCRUDHandler_Insert_WritePermissionsError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "write", gomock.Any()).
		Return(nil, access.ErrReservedTable)
	h := NewCRUDHandler(Deps{
		Builder: mockgen.NewMockRequestQueryBuilder(ctrl), SQL: mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl), DB: mockDatabaseRegistry(ctrl), Perms: perms,
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, crudRequestBody(http.MethodPost, "/prest-test/public/test", `{"name":"x"}`, testTableVars))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCRUDHandler_Update_RejectsUnwritableField(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	h := NewCRUDHandler(Deps{
		Builder: mockgen.NewMockRequestQueryBuilder(ctrl), SQL: mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl), DB: mockDatabaseRegistry(ctrl),
		Perms: permsFor(ctrl, []string{"name"}, []string{"name"}, true),
	})
	req := crudRequestBody(http.MethodPatch, "/prest-test/public/test?name=$eq.alice", `{"role":"superadmin"}`, testTableVars)
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "not writable")
}

func TestCRUDHandler_BatchInsert_RejectsUnwritableField(t *testing.T) {
	t.Parallel()

	for _, method := range []string{"", "copy"} {
		t.Run("batch method "+method, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			h := NewCRUDHandler(Deps{
				Builder: mockgen.NewMockRequestQueryBuilder(ctrl), SQL: mockgen.NewMockSQLBuilder(ctrl),
				Executor: mockgen.NewMockQueryExecutor(ctrl), DB: mockDatabaseRegistry(ctrl),
				Perms: permsFor(ctrl, []string{"name"}, []string{"name"}, true),
			})
			// The unwritable key only appears in the second record.
			req := crudRequestBody(http.MethodPost, "/batch/prest-test/public/test",
				`[{"name":"a"},{"name":"b","role":"admin"}]`, testTableVars)
			if method != "" {
				req.Header.Set("Prest-Batch-Method", method)
			}
			rec := httptest.NewRecorder()
			h.BatchInsert(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), "not writable")
		})
	}
}

// In multi-database mode the request's adapter (and its config) decides, not
// the handler's default permissions checker.
func TestCRUDHandler_Insert_UsesContextAdapterPerms(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	selected := mockgen.NewMockAdapter(ctrl)
	selected.EXPECT().IsRegistered("prest-test").Return(true)
	selected.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "test", "write", "").Return([]string{"name"}, nil)

	h := NewCRUDHandler(Deps{
		Perms: permsAllowAll(ctrl), SQL: mockgen.NewMockSQLBuilder(ctrl),
		Builder: mockgen.NewMockRequestQueryBuilder(ctrl), Executor: mockgen.NewMockQueryExecutor(ctrl),
		DB: mockgen.NewMockDatabaseRegistry(ctrl),
	})
	req := crudRequestBody(http.MethodPost, "/prest-test/public/test", `{"role":"admin"}`, testTableVars)
	req = req.WithContext(context.WithValue(req.Context(), pctx.AdapterKey, selected))
	rec := httptest.NewRecorder()
	h.Insert(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// GHSA-h25q-9rmj-646h (C:L): write responses only echo readable columns.
func TestCRUDHandler_Insert_ResponseFiltering(t *testing.T) {
	t.Parallel()

	const row = `{"id":1,"name":"x","role":"admin"}`
	testCases := []struct {
		description string
		read        []string
		readGranted bool
		want        string
	}{
		{"read allowlist drops role", []string{"id", "name"}, true, `{"id":1,"name":"x"}`},
		{"write-only caller sees no column", []string{"*"}, false, `{}`},
		{"read wildcard leaves the response untouched", []string{"*"}, true, row},
		{"read grant without fields echoes nothing", nil, true, `{}`},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			rec := insertReachesBuilder(t, ctrl, permsFor(ctrl, []string{"*"}, tc.read, tc.readGranted), `{"name":"x"}`, row)

			require.Equal(t, http.StatusCreated, rec.Code)
			require.Equal(t, tc.want, rec.Body.String())
		})
	}
}

// A read-permission error must surface before the write executes.
func TestCRUDHandler_Insert_ReadPermissionsErrorBeforeExec(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "write", gomock.Any()).Return([]string{"*"}, nil)
	perms.EXPECT().TablePermissions(gomock.Any(), gomock.Any(), gomock.Any(), "read", gomock.Any()).Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "read", gomock.Any()).
		Return(nil, errors.New("bad read rule"))
	h := NewCRUDHandler(Deps{
		Builder: mockgen.NewMockRequestQueryBuilder(ctrl), SQL: mockgen.NewMockSQLBuilder(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl), DB: mockDatabaseRegistry(ctrl), Perms: perms,
	})
	rec := httptest.NewRecorder()
	h.Insert(rec, crudRequestBody(http.MethodPost, "/prest-test/public/test", `{"name":"x"}`, testTableVars))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

// A response that cannot be filtered is never sent as-is.
func TestCRUDHandler_Insert_UnfilterableResponse(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	rec := insertReachesBuilder(t, ctrl, permsFor(ctrl, []string{"*"}, []string{"id"}, true), `{"name":"x"}`, `not json`)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "could not filter response columns")
	require.NotContains(t, rec.Body.String(), "not json")
}

func TestCRUDHandler_BatchInsert_ResponseFiltering(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		copyMethod  bool
		response    string
		want        string
	}{
		{"values filters every row", false, `[{"id":1,"role":"a"},{"id":2,"role":"b"}]`, `[{"id":1},{"id":2}]`},
		{"copy empty body passes through", true, ``, ``},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			builder := mockgen.NewMockRequestQueryBuilder(ctrl)
			builder.EXPECT().ParseBatchInsertRequest(gomock.Any()).Return(`name`, "", []interface{}{"a"}, nil)
			scanner := mockgen.NewMockScanner(ctrl)
			scanner.EXPECT().Err().Return(nil)
			scanner.EXPECT().Bytes().Return([]byte(tc.response))
			executor := mockgen.NewMockQueryExecutor(ctrl)
			sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
			if tc.copyMethod {
				executor.EXPECT().BatchInsertCopyCtx(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)
			} else {
				sqlBuilder.EXPECT().InsertSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`INSERT`)
				executor.EXPECT().BatchInsertValuesCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)
			}

			h := NewCRUDHandler(Deps{
				Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl),
				Perms: permsFor(ctrl, []string{"*"}, []string{"id"}, true),
			})
			req := crudRequestBody(http.MethodPost, "/batch/prest-test/public/test", `[{"name":"a"}]`, testTableVars)
			if tc.copyMethod {
				req.Header.Set("Prest-Batch-Method", "copy")
			}
			rec := httptest.NewRecorder()
			h.BatchInsert(rec, req)

			require.Equal(t, http.StatusCreated, rec.Code)
			require.Equal(t, tc.want, rec.Body.String())
		})
	}
}

func TestCRUDHandler_Update_ReturningFiltered(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"y"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("id=$2", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return(`*`, nil)
	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`UPDATE test SET name=$1`)
	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"id":1,"name":"y","role":"a"}]`))
	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().UpdateCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{
		Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl),
		Perms: permsFor(ctrl, []string{"name"}, []string{"name"}, true),
	})
	rec := httptest.NewRecorder()
	h.Update(rec, crudRequestBody(http.MethodPatch, "/prest-test/public/test?id=1&_returning=*", `{"name":"y"}`, testTableVars))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, `[{"name":"y"}]`, rec.Body.String())
}

// Without _returning the response is only a row count, so the read rule is not
// consulted (strict mock: no read expectation).
func TestCRUDHandler_Update_NoReturningSkipsReadCheck(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	perms.EXPECT().FieldsPermissions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "write", gomock.Any()).Return([]string{"name"}, nil)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().SetByRequest(gomock.Any(), 1).Return(`name=$1`, []interface{}{"y"}, nil)
	builder.EXPECT().WhereByRequest(gomock.Any(), 2).Return("id=$2", []interface{}{1}, nil)
	builder.EXPECT().ReturningByRequest(gomock.Any()).Return("", nil)
	sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
	sqlBuilder.EXPECT().UpdateSQL(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(`UPDATE test SET name=$1`)
	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`{"rows_affected":1}`))
	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().UpdateCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

	h := NewCRUDHandler(Deps{Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl), Perms: perms})
	rec := httptest.NewRecorder()
	h.Update(rec, crudRequestBody(http.MethodPatch, "/prest-test/public/test?id=1", `{"name":"y"}`, testTableVars))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, `{"rows_affected":1}`, rec.Body.String())
}

func TestCRUDHandler_Delete_ReturningFiltered(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		response    string
		want        string
	}{
		{"rows lose unreadable columns", `[{"id":1,"name":"x","role":"a"}]`, `[{"name":"x"}]`},
		{"no deleted rows (null) passes through", `null`, `null`},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)

			builder := mockgen.NewMockRequestQueryBuilder(ctrl)
			builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("id=$1", []interface{}{1}, nil)
			builder.EXPECT().ReturningByRequest(gomock.Any()).Return(`*`, nil)
			sqlBuilder := mockgen.NewMockSQLBuilder(ctrl)
			sqlBuilder.EXPECT().DeleteSQL(gomock.Any(), gomock.Any(), gomock.Any()).Return(`DELETE FROM test`)
			scanner := mockgen.NewMockScanner(ctrl)
			scanner.EXPECT().Err().Return(nil)
			scanner.EXPECT().Bytes().Return([]byte(tc.response))
			executor := mockgen.NewMockQueryExecutor(ctrl)
			executor.EXPECT().DeleteCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)

			h := NewCRUDHandler(Deps{
				Builder: builder, SQL: sqlBuilder, Executor: executor, DB: mockDatabaseRegistry(ctrl),
				Perms: permsFor(ctrl, []string{"*"}, []string{"name"}, true),
			})
			rec := httptest.NewRecorder()
			h.Delete(rec, crudRequestBody(http.MethodDelete, "/prest-test/public/test?id=1&_returning=*", ``, testTableVars))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tc.want, rec.Body.String())
		})
	}
}
