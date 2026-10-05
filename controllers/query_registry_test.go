package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/stretchr/testify/require"
)

func testQueryRegistryHandler(t *testing.T, ctrl *gomock.Controller) (*QueryRegistryHandler, *mockgen.MockQueryRegistry, *mockgen.MockDatabaseRegistry) {
	t.Helper()

	registry := mockgen.NewMockQueryRegistry(ctrl)
	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	h := NewQueryRegistryHandler(Deps{QueryRegistry: registry, DB: db}, config.QueriesConf{})
	return h, registry, db
}

func queryRegistryRequest(method, path string, body []byte, vars map[string]string) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	return req.WithContext(withTestTimeout(req.Context()))
}

func queryRegistryRequestWithUser(method, path string, body []byte, vars map[string]string, username string) *http.Request {
	req := queryRegistryRequest(method, path, body, vars)
	if username != "" {
		ctx := context.WithValue(req.Context(), pctx.UserInfoKey, auth.User{Username: username})
		req = req.WithContext(ctx)
	}
	return req
}

func TestQueryRegistryHandler_List_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	queries := []adapters.StoredQuery{
		{Location: "fulltable", Name: "get_all", ReadSQL: "SELECT 1"},
	}
	registry.EXPECT().
		ListQueries(gomock.Any(), "", "").
		Return(queries, nil)

	rec := httptest.NewRecorder()
	h.List(rec, queryRegistryRequest(http.MethodGet, "/_QUERIES/registry", nil, nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got []adapters.StoredQuery
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Equal(t, queries, got)
}

func TestQueryRegistryHandler_List_WithFilters(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		ListQueries(gomock.Any(), "other-db", "itest").
		Return([]adapters.StoredQuery{}, nil)

	rec := httptest.NewRecorder()
	h.List(rec, queryRegistryRequest(http.MethodGet, "/_QUERIES/registry?database=other-db&location=itest", nil, nil))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestQueryRegistryHandler_List_UsesContextRegistryForAlias(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	selectedDB := mockgen.NewMockAdapter(ctrl)
	selectedReg := mockgen.NewMockQueryRegistry(ctrl)
	selectedDB.EXPECT().GetDatabase().Return("shop")
	selectedReg.EXPECT().
		ListQueries(gomock.Any(), "shop", "itest").
		Return([]adapters.StoredQuery{{DatabaseAlias: "shop", Location: "itest", Name: "sample"}}, nil)

	selected := struct {
		adapters.Adapter
		adapters.QueryRegistry
	}{Adapter: selectedDB, QueryRegistry: selectedReg}

	h := NewQueryRegistryHandler(Deps{
		QueryRegistry: mockgen.NewMockQueryRegistry(ctrl),
		DB:            mockgen.NewMockDatabaseRegistry(ctrl),
	}, config.QueriesConf{})

	req := queryRegistryRequest(http.MethodGet, "/_QUERIES/registry/shop?location=itest", nil, map[string]string{
		"database": "shop",
	})
	req = req.WithContext(context.WithValue(req.Context(), pctx.AdapterKey, selected))
	rec := httptest.NewRecorder()
	h.List(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"database":"shop"`)
}

func TestQueryRegistryHandler_List_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		ListQueries(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("list failed"))

	rec := httptest.NewRecorder()
	h.List(rec, queryRegistryRequest(http.MethodGet, "/_QUERIES/registry", nil, nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "list failed")
}

func TestQueryRegistryHandler_Get_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	q := adapters.StoredQuery{Location: "fulltable", Name: "get_all", ReadSQL: "SELECT 1"}
	registry.EXPECT().
		GetQuery(gomock.Any(), "", "fulltable", "get_all").
		Return(q, nil)

	rec := httptest.NewRecorder()
	h.Get(rec, queryRegistryRequest(http.MethodGet, "/_QUERIES/registry/fulltable/get_all", nil, map[string]string{
		"location": "fulltable",
		"name":     "get_all",
	}))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got adapters.StoredQuery
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Equal(t, q, got)
}

func TestQueryRegistryHandler_Get_UsesContextAdapter(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	selectedDB := mockgen.NewMockAdapter(ctrl)
	selectedReg := mockgen.NewMockQueryRegistry(ctrl)
	selectedDB.EXPECT().GetDatabase().Return("prest-test")
	q := adapters.StoredQuery{DatabaseAlias: "shop", Location: "fulltable", Name: "get_all", ReadSQL: "SELECT 1"}
	selectedReg.EXPECT().GetQuery(gomock.Any(), "shop", "fulltable", "get_all").Return(q, nil)

	selected := struct {
		adapters.Adapter
		adapters.QueryRegistry
	}{Adapter: selectedDB, QueryRegistry: selectedReg}

	h := NewQueryRegistryHandler(Deps{
		QueryRegistry: mockgen.NewMockQueryRegistry(ctrl),
		DB:            mockgen.NewMockDatabaseRegistry(ctrl),
	}, config.QueriesConf{})

	req := queryRegistryRequest(http.MethodGet, "/_QUERIES/registry/shop/fulltable/get_all", nil, map[string]string{
		"database": "shop",
		"location": "fulltable",
		"name":     "get_all",
	})
	req = req.WithContext(context.WithValue(req.Context(), pctx.AdapterKey, selected))
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"get_all"`)
}

func TestQueryRegistryHandler_Get_DatabaseFromQuery(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	q := adapters.StoredQuery{DatabaseAlias: "other-db", Location: "itest", Name: "sample"}
	registry.EXPECT().
		GetQuery(gomock.Any(), "other-db", "itest", "sample").
		Return(q, nil)

	rec := httptest.NewRecorder()
	h.Get(rec, queryRegistryRequest(http.MethodGet, "/_QUERIES/registry/itest/sample?database=other-db", nil, map[string]string{
		"location": "itest",
		"name":     "sample",
	}))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestQueryRegistryHandler_Get_DatabaseFromPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	q := adapters.StoredQuery{DatabaseAlias: "prest-test", Location: "fulltable", Name: "get_all"}
	registry.EXPECT().
		GetQuery(gomock.Any(), "prest-test", "fulltable", "get_all").
		Return(q, nil)

	rec := httptest.NewRecorder()
	h.Get(rec, queryRegistryRequest(http.MethodGet, "/_QUERIES/registry/prest-test/fulltable/get_all", nil, map[string]string{
		"database": "prest-test",
		"location": "fulltable",
		"name":     "get_all",
	}))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestQueryRegistryHandler_Get_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		GetQuery(gomock.Any(), gomock.Any(), "missing", "nope").
		Return(adapters.StoredQuery{}, errors.New("not found"))

	rec := httptest.NewRecorder()
	h.Get(rec, queryRegistryRequest(http.MethodGet, "/_QUERIES/registry/missing/nope", nil, map[string]string{
		"location": "missing",
		"name":     "nope",
	}))

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "not found")
}

func TestQueryRegistryHandler_Create_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	body := []byte(`{"location":"itest","name":"sample","read_sql":"SELECT 1"}`)
	registry.EXPECT().
		UpsertQuery(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q adapters.StoredQuery) error {
			require.Equal(t, "itest", q.Location)
			require.Equal(t, "sample", q.Name)
			require.Equal(t, "SELECT 1", q.ReadSQL)
			require.Equal(t, "admin@test", q.CreatedBy)
			return nil
		})

	rec := httptest.NewRecorder()
	h.Create(rec, queryRegistryRequestWithUser(http.MethodPost, "/_QUERIES/registry", body, nil, "admin@test"))

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got adapters.StoredQuery
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Equal(t, "itest", got.Location)
	require.Equal(t, "sample", got.Name)
	require.Equal(t, "admin@test", got.CreatedBy)
}

func TestQueryRegistryHandler_Create_UsesContextRegistryForAlias(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	selectedDB := mockgen.NewMockAdapter(ctrl)
	selectedReg := mockgen.NewMockQueryRegistry(ctrl)
	selectedDB.EXPECT().GetDatabase().Return("shop")
	selectedReg.EXPECT().
		UpsertQuery(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q adapters.StoredQuery) error {
			require.Equal(t, "shop", q.DatabaseAlias)
			require.Equal(t, "itest", q.Location)
			require.Equal(t, "sample", q.Name)
			require.Equal(t, "admin@test", q.CreatedBy)
			return nil
		})

	selected := struct {
		adapters.Adapter
		adapters.QueryRegistry
	}{Adapter: selectedDB, QueryRegistry: selectedReg}

	h := NewQueryRegistryHandler(Deps{
		QueryRegistry: mockgen.NewMockQueryRegistry(ctrl),
		DB:            mockgen.NewMockDatabaseRegistry(ctrl),
	}, config.QueriesConf{})

	body := []byte(`{"location":"itest","name":"sample","read_sql":"SELECT 1","database":"other"}`)
	req := queryRegistryRequestWithUser(http.MethodPost, "/_QUERIES/registry/shop", body, map[string]string{
		"database": "shop",
	}, "admin@test")
	req = req.WithContext(context.WithValue(req.Context(), pctx.AdapterKey, selected))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var got adapters.StoredQuery
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Equal(t, "shop", got.DatabaseAlias)
}

func TestQueryRegistryHandler_Create_InvalidJSON(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, _, _ := testQueryRegistryHandler(t, ctrl)

	rec := httptest.NewRecorder()
	h.Create(rec, queryRegistryRequest(http.MethodPost, "/_QUERIES/registry", []byte(`{invalid`), nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid json body")
}

func TestQueryRegistryHandler_Create_UpsertError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		UpsertQuery(gomock.Any(), gomock.Any()).
		Return(errors.New("upsert failed"))

	rec := httptest.NewRecorder()
	body := []byte(`{"location":"itest","name":"sample","read_sql":"SELECT 1"}`)
	h.Create(rec, queryRegistryRequest(http.MethodPost, "/_QUERIES/registry", body, nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "upsert failed")
}

func TestQueryRegistryHandler_Update_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	body := []byte(`{"read_sql":"SELECT 2"}`)
	registry.EXPECT().
		UpsertQuery(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q adapters.StoredQuery) error {
			require.Equal(t, "itest", q.Location)
			require.Equal(t, "sample", q.Name)
			require.Equal(t, "SELECT 2", q.ReadSQL)
			require.Equal(t, "admin@test", q.CreatedBy)
			return nil
		})

	rec := httptest.NewRecorder()
	h.Update(rec, queryRegistryRequestWithUser(http.MethodPut, "/_QUERIES/registry/itest/sample", body, map[string]string{
		"location": "itest",
		"name":     "sample",
	}, "admin@test"))

	require.Equal(t, http.StatusOK, rec.Code)

	var got adapters.StoredQuery
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Equal(t, "itest", got.Location)
	require.Equal(t, "sample", got.Name)
	require.Equal(t, "SELECT 2", got.ReadSQL)
}

func TestQueryRegistryHandler_Update_OverridesPathVars(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	body := []byte(`{"location":"ignored","name":"ignored","read_sql":"SELECT 3"}`)
	registry.EXPECT().
		UpsertQuery(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q adapters.StoredQuery) error {
			require.Equal(t, "itest", q.Location)
			require.Equal(t, "sample", q.Name)
			return nil
		})

	rec := httptest.NewRecorder()
	h.Update(rec, queryRegistryRequest(http.MethodPut, "/_QUERIES/registry/itest/sample", body, map[string]string{
		"location": "itest",
		"name":     "sample",
	}))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestQueryRegistryHandler_Update_DatabaseFromPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	body := []byte(`{"read_sql":"SELECT 4"}`)
	registry.EXPECT().
		UpsertQuery(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q adapters.StoredQuery) error {
			require.Equal(t, "other-db", q.DatabaseAlias)
			return nil
		})

	rec := httptest.NewRecorder()
	h.Update(rec, queryRegistryRequest(http.MethodPut, "/_QUERIES/registry/other-db/itest/sample", body, map[string]string{
		"database": "other-db",
		"location": "itest",
		"name":     "sample",
	}))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestQueryRegistryHandler_Update_InvalidJSON(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, _, _ := testQueryRegistryHandler(t, ctrl)

	rec := httptest.NewRecorder()
	h.Update(rec, queryRegistryRequest(http.MethodPut, "/_QUERIES/registry/itest/sample", []byte(`not-json`), map[string]string{
		"location": "itest",
		"name":     "sample",
	}))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid json body")
}

func TestQueryRegistryHandler_Update_UpsertError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		UpsertQuery(gomock.Any(), gomock.Any()).
		Return(errors.New("update failed"))

	rec := httptest.NewRecorder()
	body := []byte(`{"read_sql":"SELECT 2"}`)
	h.Update(rec, queryRegistryRequest(http.MethodPut, "/_QUERIES/registry/itest/sample", body, map[string]string{
		"location": "itest",
		"name":     "sample",
	}))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "update failed")
}

func TestQueryRegistryHandler_Delete_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		DeleteQuery(gomock.Any(), "", "itest", "sample").
		Return(nil)

	rec := httptest.NewRecorder()
	h.Delete(rec, queryRegistryRequest(http.MethodDelete, "/_QUERIES/registry/itest/sample", nil, map[string]string{
		"location": "itest",
		"name":     "sample",
	}))

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
}

func TestQueryRegistryHandler_Delete_DatabaseFromQuery(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		DeleteQuery(gomock.Any(), "other-db", "itest", "sample").
		Return(nil)

	rec := httptest.NewRecorder()
	h.Delete(rec, queryRegistryRequest(http.MethodDelete, "/_QUERIES/registry/itest/sample?database=other-db", nil, map[string]string{
		"location": "itest",
		"name":     "sample",
	}))

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestQueryRegistryHandler_Delete_DatabaseFromPath(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		DeleteQuery(gomock.Any(), "prest-test", "itest", "sample").
		Return(nil)

	rec := httptest.NewRecorder()
	h.Delete(rec, queryRegistryRequest(http.MethodDelete, "/_QUERIES/registry/prest-test/itest/sample", nil, map[string]string{
		"database": "prest-test",
		"location": "itest",
		"name":     "sample",
	}))

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestQueryRegistryHandler_Delete_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, registry, _ := testQueryRegistryHandler(t, ctrl)
	registry.EXPECT().
		DeleteQuery(gomock.Any(), gomock.Any(), "missing", "nope").
		Return(errors.New("not found"))

	rec := httptest.NewRecorder()
	h.Delete(rec, queryRegistryRequest(http.MethodDelete, "/_QUERIES/registry/missing/nope", nil, map[string]string{
		"location": "missing",
		"name":     "nope",
	}))

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "not found")
}

func TestQueryRegistryHandler_decodeBody_ValidJSON(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, _, _ := testQueryRegistryHandler(t, ctrl)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/_QUERIES/registry", bytes.NewReader([]byte(`{"location":"itest","name":"sample"}`)))

	q, err := h.decodeBody(rec, req)
	require.NoError(t, err)
	require.Equal(t, "itest", q.Location)
	require.Equal(t, "sample", q.Name)
}

func TestQueryRegistryHandler_decodeBody_InvalidJSON(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h, _, _ := testQueryRegistryHandler(t, ctrl)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/_QUERIES/registry", bytes.NewReader([]byte(`{bad`)))

	_, err := h.decodeBody(rec, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid json body")
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	payload := map[string]string{
		"message": "ok <html>",
		"status":  "done",
	}
	writeJSON(rec, payload)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Equal(t, `{"message":"ok <html>","status":"done"}`+"\n", rec.Body.String())
}
