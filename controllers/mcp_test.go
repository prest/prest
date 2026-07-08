package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/prest/prest/v2/adapters/mockgen"
	"github.com/stretchr/testify/require"
)

func TestMCPHandler_GetDiscovery(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return(`SELECT n.nspname as "schema", c.relname as "name" FROM pg_catalog.pg_class c`)
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users","type":"table"}]`))

	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true).AnyTimes()
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"*"}, nil).AnyTimes()

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner).AnyTimes()
	showScanner.EXPECT().Err().Return(nil).AnyTimes()
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1},{"column_name":"name","data_type":"text","position":2}]`)).AnyTimes()

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_mcp", nil)

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest.select.prest-test.public.users")
	require.Contains(t, rec.Body.String(), "order_by")
	require.Contains(t, rec.Body.String(), "filters")
}

func TestMCPHandler_HandleRPC_InvalidJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	h := NewMCPHandler(Deps{})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", bytes.NewBufferString("{")))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid request")
}

func TestMCPHandler_HandleRPC_MissingMethod(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	h := NewMCPHandler(Deps{})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1}`)
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "missing method")
}

func TestMCPHandler_SelectTableWithFiltersAndOrder(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id", "name"}, nil)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[
		{"column_name":"id","data_type":"integer","position":1},
		{"column_name":"name","data_type":"text","position":2}
	]`))

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(
		gomock.Any(),
		`SELECT "id", "name" FROM "public"."users" WHERE "name" = $1 ORDER BY "id" DESC LIMIT 10 OFFSET 5`,
		"Alice",
	).Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"id":1,"name":"Alice"}]`))

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	result, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test",
		Schema:   "public",
		Table:    "users",
		Columns:  []string{"id", "name"},
		Filters:  map[string]any{"name": "Alice"},
		OrderBy:  []string{"-id"},
		Limit:    10,
		Offset:   5,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.(mcpSelectResult).Count)
}

func TestMCPHandler_ListDatabasesUsesAliases(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().Aliases().Return([]string{"prest-test", "secondary-db"})
	db.EXPECT().PhysicalName("prest-test").Return("prest-test")
	db.EXPECT().PhysicalName("secondary-db").Return("secondary-cluster")

	h := NewMCPHandler(Deps{DB: db, PGDatabase: "prest-test"})
	rows, err := h.listDatabases(httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.NoError(t, err)
	payload := rows.([]map[string]any)
	require.Len(t, payload, 2)
	require.Equal(t, "secondary-cluster", payload[1]["physical_name"])
}

func TestMCPHandler_AccessDeniedFiltersTools(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return(`SELECT n.nspname as "schema", c.relname as "name" FROM pg_catalog.pg_class c`)
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users","type":"table"}]`))
	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))

	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(false).AnyTimes()

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	tools, err := h.tools(httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.NoError(t, err)

	for _, tool := range tools {
		require.NotContains(t, tool.Name, "prest.select.prest-test.public.users")
	}
}

func TestMCPHandler_ToolsCallRejectsUnknownTool(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.drop_table"}}`)
	req := httptest.NewRequest(http.MethodPost, "/_mcp", body)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported tool")
}

func TestParseSelectToolName(t *testing.T) {
	t.Parallel()

	args, err := parseSelectToolName("prest.select.prest-test.public.users")
	require.NoError(t, err)
	require.Equal(t, "prest-test", args.Database)
	require.Equal(t, "public", args.Schema)
	require.Equal(t, "users", args.Table)

	_, err = parseSelectToolName("prest.select.invalid")
	require.Error(t, err)
}

func TestDecodeJSONRows(t *testing.T) {
	t.Parallel()

	rows, err := decodeJSONRows([]byte(`[{"id":1},{"id":2}]`))
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestDecodeWithNumbers(t *testing.T) {
	t.Parallel()

	var out map[string]any
	err := decodeWithNumbers([]byte(`{"n":1}`), &out)
	require.NoError(t, err)
	require.Equal(t, "1", out["n"].(json.Number).String())
}

func TestBuildFilterClause(t *testing.T) {
	t.Parallel()

	columns := map[string]mcpColumn{"id": {Name: "id"}, "name": {Name: "name"}}
	clause, values, err := buildFilterClause(map[string]any{"id": float64(1), "name": "Alice"}, columns)
	require.NoError(t, err)
	require.Contains(t, clause, `"id" = $1`)
	require.Contains(t, clause, `"name" = $2`)
	require.Len(t, values, 2)

	_, _, err = buildFilterClause(map[string]any{"unknown": 1}, columns)
	require.Error(t, err)
}

func TestBuildOrderClause(t *testing.T) {
	t.Parallel()

	columns := map[string]mcpColumn{"id": {Name: "id"}, "name": {Name: "name"}}
	clause, err := buildOrderClause([]string{"-id", "name"}, columns)
	require.NoError(t, err)
	require.Equal(t, `ORDER BY "id" DESC, "name" ASC`, clause)

	_, err = buildOrderClause([]string{"missing"}, columns)
	require.Error(t, err)
}

func TestMCPHandler_HandlerAndWriteHelpers(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	require.NotNil(t, h.Handler())

	rec := httptest.NewRecorder()
	h.writeJSON(rec, http.StatusOK, map[string]any{"ok": true})
	require.Equal(t, http.StatusOK, rec.Code)

	recErr := httptest.NewRecorder()
	h.writeRPCError(recErr, json.RawMessage("1"), http.StatusBadRequest, "boom", errors.New("x"))
	require.Equal(t, http.StatusBadRequest, recErr.Code)
	require.Contains(t, recErr.Body.String(), "boom")
}
