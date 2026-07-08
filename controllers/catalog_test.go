package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestCatalogHandler_ListDatabases_BuilderError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, errors.New("invalid where"))

	h := NewCatalogHandler(Deps{
		Builder:  builder,
		Catalog:  mockgen.NewMockCatalogQuerier(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		DB:       mockgen.NewMockDatabaseRegistry(ctrl),
	})

	req := httptest.NewRequest(http.MethodGet, "/databases?bad=1", nil)
	rec := httptest.NewRecorder()
	h.ListDatabases(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCatalogHandler_ListDatabases_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)

	catalog.EXPECT().DatabaseWhere("").Return("")
	catalog.EXPECT().DatabaseClause(gomock.Any()).Return("SELECT datname FROM pg_database", false)
	catalog.EXPECT().DatabaseOrderBy("", false).Return("")
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)
	executor.EXPECT().Query(gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"datname":"prest-test"}]`))

	h := NewCatalogHandler(Deps{
		Catalog:  catalog,
		Builder:  builder,
		Executor: executor,
		DB:       mockgen.NewMockDatabaseRegistry(ctrl),
	})

	req := httptest.NewRequest(http.MethodGet, "/databases", nil)
	rec := httptest.NewRecorder()
	h.ListDatabases(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest-test")
}

func TestCatalogHandler_ListDatabases_WithOptions(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)

	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("datname=$1", []interface{}{"prest"}, nil)
	catalog.EXPECT().DatabaseWhere("datname=$1").Return("WHERE datistemplate = false AND datname=$1")
	catalog.EXPECT().DatabaseClause(gomock.Any()).Return("SELECT datname FROM pg_database", false)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("DISTINCT", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("ORDER BY datname DESC", nil)
	catalog.EXPECT().DatabaseOrderBy("ORDER BY datname DESC", false).Return("ORDER BY datname DESC")
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("LIMIT 5", nil)
	executor.EXPECT().Query(gomock.Any(), "prest").Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"datname":"prest"}]`))

	h := NewCatalogHandler(Deps{Catalog: catalog, Builder: builder, Executor: executor})
	rec := httptest.NewRecorder()
	h.ListDatabases(rec, httptest.NewRequest(http.MethodGet, "/databases?datname=$eq.prest&_distinct=true&_order=datname&_page=1", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest")
}

func TestCatalogHandler_ListDatabases_QueryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)

	catalog.EXPECT().DatabaseWhere("").Return("")
	catalog.EXPECT().DatabaseClause(gomock.Any()).Return("SELECT datname FROM pg_database", false)
	catalog.EXPECT().DatabaseOrderBy("", false).Return("")
	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)
	executor.EXPECT().Query(gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(errors.New("query failed")).Times(2)

	h := NewCatalogHandler(Deps{Catalog: catalog, Builder: builder, Executor: executor})
	rec := httptest.NewRecorder()
	h.ListDatabases(rec, httptest.NewRequest(http.MethodGet, "/databases", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "query failed")
}

func TestCatalogHandler_ListSchemas_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)

	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("nspname=$1", []interface{}{"public"}, nil)
	catalog.EXPECT().SchemaClause(gomock.Any()).Return("SELECT nspname FROM pg_namespace", false)
	catalog.EXPECT().SchemaOrderBy("", false).Return("")
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)
	executor.EXPECT().Query(gomock.Any(), "public").Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"nspname":"public"}]`))

	h := NewCatalogHandler(Deps{Catalog: catalog, Builder: builder, Executor: executor})
	rec := httptest.NewRecorder()
	h.ListSchemas(rec, httptest.NewRequest(http.MethodGet, "/schemas", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "public")
}

func TestCatalogHandler_ListSchemas_WithOptions(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)

	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("nspname=$1", []interface{}{"public"}, nil)
	catalog.EXPECT().SchemaClause(gomock.Any()).Return("SELECT nspname FROM pg_namespace", false)
	builder.EXPECT().DistinctClause(gomock.Any()).Return("DISTINCT", nil)
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("ORDER BY nspname DESC", nil)
	catalog.EXPECT().SchemaOrderBy("ORDER BY nspname DESC", false).Return("ORDER BY nspname DESC")
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("LIMIT 10", nil)
	executor.EXPECT().Query(gomock.Any(), "public").Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"nspname":"public"}]`))

	h := NewCatalogHandler(Deps{Catalog: catalog, Builder: builder, Executor: executor})
	rec := httptest.NewRecorder()
	h.ListSchemas(rec, httptest.NewRequest(http.MethodGet, "/schemas?nspname=$eq.public&_distinct=true&_order=nspname&_page=1", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "public")
}

func TestCatalogHandler_ListTables_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)

	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("", nil, nil)
	catalog.EXPECT().TableWhere("").Return("")
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	catalog.EXPECT().TableOrderBy("").Return("")
	catalog.EXPECT().TableClause().Return("SELECT tablename FROM pg_tables")
	builder.EXPECT().DistinctClause(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)
	executor.EXPECT().Query(gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"tablename":"users"}]`))

	h := NewCatalogHandler(Deps{Catalog: catalog, Builder: builder, Executor: executor})
	rec := httptest.NewRecorder()
	h.ListTables(rec, httptest.NewRequest(http.MethodGet, "/tables", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "users")
}

func TestCatalogHandler_ListTables_WithOptions(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)

	builder.EXPECT().WhereByRequest(gomock.Any(), 1).Return("tablename=$1", []interface{}{"users"}, nil)
	catalog.EXPECT().TableWhere("tablename=$1").Return("WHERE has_schema_privilege(n.nspname, 'USAGE') AND tablename=$1")
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("ORDER BY tablename DESC", nil)
	catalog.EXPECT().TableOrderBy("ORDER BY tablename DESC").Return("ORDER BY tablename DESC")
	catalog.EXPECT().TableClause().Return("SELECT tablename FROM pg_tables")
	builder.EXPECT().DistinctClause(gomock.Any()).Return("DISTINCT", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("LIMIT 5", nil)
	executor.EXPECT().Query(gomock.Any(), "users").Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"tablename":"users"}]`))

	h := NewCatalogHandler(Deps{Catalog: catalog, Builder: builder, Executor: executor})
	rec := httptest.NewRecorder()
	h.ListTables(rec, httptest.NewRequest(http.MethodGet, "/tables?tablename=$eq.users&_distinct=true&_order=tablename&_page=1", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "users")
}

func TestCatalogHandler_ListTablesByDatabaseAndSchema_InvalidPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockDatabaseRegistry(ctrl)

	h := NewCatalogHandler(Deps{
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		Catalog:  mockgen.NewMockCatalogQuerier(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
		DB:       db,
	})

	req := httptest.NewRequest(http.MethodGet, "/prest-test/bad@schema", nil)
	req = mux.SetURLVars(req, map[string]string{"database": "prest-test", "schema": "bad@schema"})
	rec := httptest.NewRecorder()
	h.ListTablesByDatabaseAndSchema(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCatalogHandler_ListTablesByDatabaseAndSchema_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)
	db := mockDatabaseRegistry(ctrl)

	builder.EXPECT().WhereByRequest(gomock.Any(), 3).Return("", nil, nil)
	catalog.EXPECT().SchemaTablesWhere("").Return(" AND schemaname=$2")
	catalog.EXPECT().SchemaTablesClause().Return("SELECT tablename FROM pg_tables WHERE table_catalog=$1")
	catalog.EXPECT().SchemaTablesOrderBy("").Return("")
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("", nil)
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("", nil)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), "prest-test", "public").Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"tablename":"users"}]`))

	h := NewCatalogHandler(Deps{Catalog: catalog, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public", map[string]string{"database": "prest-test", "schema": "public"})
	rec := httptest.NewRecorder()
	h.ListTablesByDatabaseAndSchema(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "users")
}

func TestCatalogHandler_ListTablesByDatabaseAndSchema_WithOptions(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	builder := mockgen.NewMockRequestQueryBuilder(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)
	db := mockDatabaseRegistry(ctrl)

	builder.EXPECT().WhereByRequest(gomock.Any(), 3).Return("tablename=$1", []interface{}{"users"}, nil)
	catalog.EXPECT().SchemaTablesWhere("tablename=$1").Return(" AND tablename=$1")
	catalog.EXPECT().SchemaTablesClause().Return("SELECT tablename FROM pg_tables WHERE table_catalog=$1")
	builder.EXPECT().OrderByRequest(gomock.Any()).Return("ORDER BY tablename DESC", nil)
	catalog.EXPECT().SchemaTablesOrderBy("ORDER BY tablename DESC").Return("ORDER BY tablename DESC")
	builder.EXPECT().PaginateIfPossible(gomock.Any()).Return("LIMIT 5", nil)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), "prest-test", "public", "users").Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"tablename":"users"}]`))

	h := NewCatalogHandler(Deps{Catalog: catalog, Builder: builder, Executor: executor, DB: db})
	req := crudRequest(http.MethodGet, "/prest-test/public", map[string]string{"database": "prest-test", "schema": "public"})
	rec := httptest.NewRecorder()
	h.ListTablesByDatabaseAndSchema(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "users")
}

func TestCatalogHandler_ListTablesByDatabaseAndSchema_UnregisteredDatabase(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("other").Return(false)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewCatalogHandler(Deps{
		DB:       db,
		SingleDB: true,
		Builder:  mockgen.NewMockRequestQueryBuilder(ctrl),
		Catalog:  mockgen.NewMockCatalogQuerier(ctrl),
		Executor: mockgen.NewMockQueryExecutor(ctrl),
	})

	req := mux.SetURLVars(
		httptest.NewRequest(http.MethodGet, "/other/public", nil),
		map[string]string{"database": "other", "schema": "public"},
	)
	rec := httptest.NewRecorder()
	h.ListTablesByDatabaseAndSchema(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), fmt.Sprintf("database not registered: %v", "other"))
}
