package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestCatalogHandler_ListDatabases_BuilderError(t *testing.T) {
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

func TestCatalogHandler_ListTablesByDatabaseAndSchema_InvalidPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

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

func TestCatalogHandler_ListDatabases_Success(t *testing.T) {
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
	executor.EXPECT().Query(gomock.Any(), gomock.Any()).Return(scanner)
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
