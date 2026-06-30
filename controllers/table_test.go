package controllers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestTableHandler_Show_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id"}]`))

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewTableHandler(executor, db, false)
	req := crudRequest(http.MethodGet, "/prest-test/public/users", map[string]string{
		"database": "prest-test", "schema": "public", "table": "users",
	})
	rec := httptest.NewRecorder()
	h.Show(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "id")
}

func TestTableHandler_Show_InvalidPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewTableHandler(mockgen.NewMockQueryExecutor(ctrl), db, false)
	req := crudRequest(http.MethodGet, "/prest-test/bad@schema/users", map[string]string{
		"database": "prest-test", "schema": "bad@schema", "table": "users",
	})
	rec := httptest.NewRecorder()
	h.Show(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid identifier")
}

func TestTableHandler_Show_QueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	scanner := mockgen.NewMockScanner(ctrl)
	scanner.EXPECT().Err().Return(errors.New("schema error")).Times(2)

	executor := mockgen.NewMockQueryExecutor(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(scanner)

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewTableHandler(executor, db, false)
	req := crudRequest(http.MethodGet, "/prest-test/public/users", map[string]string{
		"database": "prest-test", "schema": "public", "table": "users",
	})
	rec := httptest.NewRecorder()
	h.Show(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "schema error")
}
