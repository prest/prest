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
	"github.com/prest/prest/v2/adapters/mockgen"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"
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

func TestMCPHandler_ListSchemasDoesNotLeakWhenAllFiltered(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().SchemaClause(gomock.Any()).Return("SELECT schema", false)
	catalog.EXPECT().SchemaOrderBy("", false).Return("")
	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	schemaScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT schema ").Return(schemaScanner)
	schemaScanner.EXPECT().Err().Return(nil)
	schemaScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public"},{"schema":"secret"}]`))

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT table  ").Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users","type":"table"}]`))

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))

	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(false)

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	result, err := h.listSchemas(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpListSchemasArgs{Database: "prest-test"})
	require.NoError(t, err)
	require.Empty(t, result.([]map[string]any))
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

func TestMCPHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/_mcp", nil))

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Contains(t, rec.Body.String(), "method not allowed")
}

func TestMCPHandler_DispatchRPC_Initialize(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	result, err := h.dispatchRPC(httptest.NewRequest(http.MethodPost, "/_mcp", nil), "initialize", nil)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMCPHandler_DispatchRPC_ToolsList(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner).AnyTimes()
	tableScanner.EXPECT().Err().Return(nil).AnyTimes()
	tableScanner.EXPECT().Bytes().Return([]byte(`[]`)).AnyTimes()

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})
	result, err := h.dispatchRPC(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "tools/list", nil)
	require.NoError(t, err)
	payload := result.(map[string]any)
	require.NotEmpty(t, payload["tools"])
}

func TestMCPHandler_DispatchRPC_UnsupportedMethod(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	_, err := h.dispatchRPC(httptest.NewRequest(http.MethodPost, "/_mcp", nil), "unknown/method", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported method")
}

func TestMCPHandler_RPC_ListDatabases(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().Aliases().Return([]string{"prest-test"}).AnyTimes()
	db.EXPECT().PhysicalName("prest-test").Return("prest-test")

	h := NewMCPHandler(Deps{DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.list_databases"}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest-test")
}

func TestMCPHandler_RPC_ListSchemas(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog, executor, perms, db := setupSchemaListMocks(ctrl)

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.list_schemas","arguments":{"database":"prest-test"}}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "public")
}

func TestMCPHandler_RPC_ListTables(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog, executor, perms, db := setupTableListMocks(ctrl, "")

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.list_tables","arguments":{"database":"prest-test"}}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "users")
}

func TestMCPHandler_RPC_ListTablesWithSchema(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog, executor, perms, db := setupTableListMocks(ctrl, "public")

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.list_tables","arguments":{"database":"prest-test","schema":"public"}}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "users")
}

func TestMCPHandler_RPC_DescribeTable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))

	h := NewMCPHandler(Deps{Executor: executor, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.describe_table","arguments":{"database":"prest-test","schema":"public","table":"users"}}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"id"`)
}

func TestMCPHandler_RPC_SelectTable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor, perms, db := setupSelectMocks(ctrl)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.select_table","arguments":{"database":"prest-test","schema":"public","table":"users","limit":5}}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"count":1`)
}

func TestMCPHandler_RPC_SchemaAwareSelectTool(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor, perms, db := setupSelectMocks(ctrl)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.select.prest-test.public.users"}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"count":1`)
}

func TestMCPHandler_RPC_SchemaAwareSelectToolWithOverrides(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor, perms, db := setupSelectMocks(ctrl)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.select.prest-test.public.users","arguments":{"columns":["id"],"limit":10}}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"count":1`)
}

func TestMCPHandler_ListDatabasesViaCatalog(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)

	catalog.EXPECT().DatabaseClause(gomock.Any()).Return("SELECT datname", true)
	catalog.EXPECT().DatabaseWhere("").Return("WHERE true")
	catalog.EXPECT().DatabaseOrderBy("", true).Return("ORDER BY datname")

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT datname WHERE true ORDER BY datname").Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"datname":"prest-test"}]`))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor})
	rows, err := h.listDatabases(httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.NoError(t, err)
	require.Len(t, rows.([]map[string]any), 1)
}

func TestMCPHandler_SelectTable_NoPermission(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(false)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	_, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test", Schema: "public", Table: "users",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission")
}

func TestMCPHandler_SelectTable_UnsupportedColumn(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	_, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test", Schema: "public", Table: "users", Columns: []string{"missing"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported column")
}

func TestMCPHandler_SelectTable_LimitClampAndDuplicateColumns(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), `SELECT "id" FROM "public"."users" LIMIT 100 OFFSET 0`, gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[]`))

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	result, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test", Schema: "public", Table: "users",
		Columns: []string{"id", "id"}, Limit: 500, Offset: -1,
	})
	require.NoError(t, err)
	require.Equal(t, 0, result.(mcpSelectResult).Count)
}

func TestMCPHandler_FilterAccessibleTables_SkipsNonQueryable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewMCPHandler(Deps{PGDatabase: "prest-test"})
	rows := []map[string]any{
		{"schema": "public", "name": "seq", "type": "sequence"},
		{"schema": "public", "name": "users", "type": "table"},
	}
	filtered, err := h.filterAccessibleTables(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "prest-test", rows)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "users", filtered[0]["name"])
}

func TestMCPHandler_DatabaseAliases_SingleDB(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{SingleDB: true, PGDatabase: "only-db"})
	require.Equal(t, []string{"only-db"}, h.databaseAliases())
}

func TestMCPHandler_DefaultDatabaseFromRegistry(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().GetDatabase().Return("registry-db")

	h := NewMCPHandler(Deps{DB: db})
	require.Equal(t, "registry-db", h.defaultDatabase())
}

func TestMCPHandler_CallTool_InvalidArguments(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	_, err := h.callTool(httptest.NewRequest(http.MethodPost, "/_mcp", nil), json.RawMessage(`{"name":"prest.list_schemas","arguments":"bad"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid list schemas arguments")
}

func TestMCPHandler_CallTool_MissingToolName(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	_, err := h.callTool(httptest.NewRequest(http.MethodPost, "/_mcp", nil), json.RawMessage(`{}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "tool name is required")
}

func setupSchemaListMocks(ctrl *gomock.Controller) (*mockgen.MockCatalogQuerier, *mockgen.MockQueryExecutor, *mockgen.MockPermissionsChecker, *mockgen.MockDatabaseRegistry) {
	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().SchemaClause(gomock.Any()).Return("SELECT schema", false)
	catalog.EXPECT().SchemaOrderBy("", false).Return("")
	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	schemaScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT schema ").Return(schemaScanner)
	schemaScanner.EXPECT().Err().Return(nil)
	schemaScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public"}]`))

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT table  ").Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users","type":"table"}]`))

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))

	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil)

	return catalog, executor, perms, db
}

func setupTableListMocks(ctrl *gomock.Controller, schema string) (*mockgen.MockCatalogQuerier, *mockgen.MockQueryExecutor, *mockgen.MockPermissionsChecker, *mockgen.MockDatabaseRegistry) {
	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	if schema != "" {
		catalog.EXPECT().SchemaTablesClause().Return("SELECT schema tables")
		catalog.EXPECT().SchemaTablesWhere("").Return(" WHERE schema=$2")
		catalog.EXPECT().SchemaTablesOrderBy("").Return("")
		scanner := mockgen.NewMockScanner(ctrl)
		executor.EXPECT().QueryCtx(gomock.Any(), "SELECT schema tables WHERE schema=$2", "prest-test", "public").Return(scanner)
		scanner.EXPECT().Err().Return(nil)
		scanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users","type":"table"}]`))
	} else {
		catalog.EXPECT().TableClause().Return("SELECT table")
		catalog.EXPECT().TableWhere("").Return("")
		catalog.EXPECT().TableOrderBy("").Return("")
		scanner := mockgen.NewMockScanner(ctrl)
		executor.EXPECT().QueryCtx(gomock.Any(), "SELECT table  ").Return(scanner)
		scanner.EXPECT().Err().Return(nil)
		scanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users","type":"table"}]`))
	}

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))

	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil)

	return catalog, executor, perms, db
}

func setupSelectMocks(ctrl *gomock.Controller) (*mockgen.MockQueryExecutor, *mockgen.MockPermissionsChecker, *mockgen.MockDatabaseRegistry) {
	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"id":1}]`))

	return executor, perms, db
}

func TestMCP_ParseSelectToolName(t *testing.T) {
	t.Parallel()

	args, err := parseSelectToolName("prest.select.prest-test.public.users")
	require.NoError(t, err)
	require.Equal(t, "prest-test", args.Database)
	require.Equal(t, "public", args.Schema)
	require.Equal(t, "users", args.Table)

	_, err = parseSelectToolName("prest.select.invalid")
	require.Error(t, err)
}

func TestMCP_DecodeJSONRows(t *testing.T) {
	t.Parallel()

	rows, err := decodeJSONRows([]byte(`[{"id":1},{"id":2}]`))
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestMCP_DecodeWithNumbers(t *testing.T) {
	t.Parallel()

	var out map[string]any
	err := decodeWithNumbers([]byte(`{"n":1}`), &out)
	require.NoError(t, err)
	require.Equal(t, "1", out["n"].(json.Number).String())
}

func TestMCP_BuildFilterClause(t *testing.T) {
	t.Parallel()

	columns := map[string]mcpColumn{"id": {Name: "id"}, "name": {Name: "name"}}
	clause, values, err := buildFilterClause(map[string]any{"id": float64(1), "name": "Alice"}, columns)
	require.NoError(t, err)
	require.Contains(t, clause, `"id" = $1`)
	require.Contains(t, clause, `"name" = $2`)
	require.Len(t, values, 2)

	nullClause, nullValues, err := buildFilterClause(map[string]any{"name": nil}, columns)
	require.NoError(t, err)
	require.Contains(t, nullClause, `"name" IS NULL`)
	require.Empty(t, nullValues)

	inClause, inValues, err := buildFilterClause(map[string]any{"id": []any{float64(1), float64(2)}}, columns)
	require.NoError(t, err)
	require.Contains(t, inClause, `"id" IN ($1, $2)`)
	require.Len(t, inValues, 2)

	_, _, err = buildFilterClause(map[string]any{"id": []any{}}, columns)
	require.Error(t, err)

	_, _, err = buildFilterClause(map[string]any{"unknown": 1}, columns)
	require.Error(t, err)
}

func TestMCP_BuildOrderClause(t *testing.T) {
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

func TestMCPHandler_RPC_InitializeViaHTTP(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"prest"`)
	require.Contains(t, rec.Body.String(), `"capabilities"`)
}

func TestMCPHandler_RPC_ToolsListViaHTTP(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner).AnyTimes()
	tableScanner.EXPECT().Err().Return(nil).AnyTimes()
	tableScanner.EXPECT().Bytes().Return([]byte(`[]`)).AnyTimes()

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"tools"`)
}

func TestMCPHandler_GetDiscovery_NoTableToolsWhenNoAliases(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_mcp", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"name":"prest"`)
	require.NotContains(t, rec.Body.String(), "prest.select.")
}

func TestMCPHandler_PhysicalDatabase_NilRegistry(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	require.Equal(t, "alias-only", h.physicalDatabase("alias-only"))
}

func TestMCPHandler_DatabaseAliases_FallbackToPGDatabase(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{PGDatabase: "fallback-db"})
	require.Equal(t, []string{"fallback-db"}, h.databaseAliases())
}

func TestMCPHandler_DatabaseAliases_EmptyWhenNoDefault(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{SingleDB: true})
	require.Nil(t, h.databaseAliases())
}

func TestMCPHandler_DatabaseAliases_RegistryWithoutAliases(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().Aliases().Return(nil)
	db.EXPECT().GetDatabase().Return("registry-default")

	h := NewMCPHandler(Deps{DB: db})
	require.Equal(t, []string{"registry-default"}, h.databaseAliases())
}

func TestMCPHandler_CurrentUserName(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/_mcp", nil)
	req = req.WithContext(withUser(req.Context(), auth.User{Username: "alice"}))
	require.Equal(t, "alice", currentUserName(req))
	require.Empty(t, currentUserName(httptest.NewRequest(http.MethodGet, "/_mcp", nil)))
}

func TestMCPHandler_ValidateToolTarget_InvalidDatabase(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("missing").Return(false)

	h := NewMCPHandler(Deps{DB: db, PGDatabase: "prest-test"})
	err := h.validateToolTarget("missing", "public", "users")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not registered")
}

func TestMCPHandler_ValidateToolTarget_InvalidPathSegment(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewMCPHandler(Deps{DB: mockDatabaseRegistry(ctrl), PGDatabase: "prest-test"})
	err := h.validateToolTarget("prest-test", "bad;schema", "users")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid identifier")
}

func TestMCPHandler_ListTables_InvalidSchema(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewMCPHandler(Deps{DB: mockDatabaseRegistry(ctrl), PGDatabase: "prest-test"})
	_, err := h.listTables(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpListTablesArgs{
		Database: "prest-test",
		Schema:   "bad;schema",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid identifier")
}

func TestMCPHandler_ListTables_UnregisteredDatabase(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("unknown").Return(false)

	h := NewMCPHandler(Deps{DB: db, PGDatabase: "prest-test"})
	_, err := h.listTables(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpListTablesArgs{Database: "unknown"})
	require.Error(t, err)
}

func TestMCPHandler_ListTables_DefaultDatabase(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog, executor, perms, db := setupTableListMocks(ctrl, "")

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	result, err := h.listTables(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpListTablesArgs{})
	require.NoError(t, err)
	require.NotEmpty(t, result.([]map[string]any))
}

func TestMCPHandler_ListSchemas_DefaultDatabaseAndNilPerms(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().SchemaClause(gomock.Any()).Return("SELECT schema", false)
	catalog.EXPECT().SchemaOrderBy("", false).Return("")
	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	schemaScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT schema ").Return(schemaScanner)
	schemaScanner.EXPECT().Err().Return(nil)
	schemaScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public"},{"schema":""}]`))

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT table  ").Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[]`))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})
	result, err := h.listSchemas(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpListSchemasArgs{})
	require.NoError(t, err)
	require.Len(t, result.([]map[string]any), 1)
}

func TestMCPHandler_ListSchemas_QueryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().SchemaClause(gomock.Any()).Return("SELECT schema", false)
	catalog.EXPECT().SchemaOrderBy("", false).Return("")

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT schema ").Return(scanner)
	scanner.EXPECT().Err().Return(errors.New("query failed"))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})
	_, err := h.listSchemas(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpListSchemasArgs{Database: "prest-test"})
	require.Error(t, err)
}

func TestMCPHandler_DescribeTable_DefaultDatabaseAndErrors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(errors.New("describe failed"))

	h := NewMCPHandler(Deps{Executor: executor, DB: db, PGDatabase: "prest-test"})
	_, err := h.describeTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpDescribeArgs{Schema: "public", Table: "users"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "describe table failed")
}

func TestMCPHandler_DescribeColumns_DecodeError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`not-json`))

	h := NewMCPHandler(Deps{Executor: executor, PGDatabase: "prest-test"})
	_, err := h.describeColumns(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "prest-test", "public", "users")
	require.Error(t, err)
}

func TestMCPHandler_SelectTable_DefaultDatabaseAndErrors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(errors.New("select failed"))

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	_, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{Schema: "public", Table: "users"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "select table failed")
}

func TestMCPHandler_SelectTable_NoPermittedColumns(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{}, nil)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	_, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test", Schema: "public", Table: "users",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission")
}

func TestMCPHandler_SelectTable_FilterAndOrderErrors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner).Times(2)
	showScanner.EXPECT().Err().Return(nil).Times(2)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`)).Times(2)
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true).Times(2)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil).Times(2)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	_, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test", Schema: "public", Table: "users",
		Filters: map[string]any{"missing": 1},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported filter column")

	_, err = h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test", Schema: "public", Table: "users",
		OrderBy: []string{"missing"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported order column")
}

func TestMCPHandler_SelectTable_DecodeResultError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil)

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`not-json`))

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	_, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test", Schema: "public", Table: "users",
	})
	require.Error(t, err)
}

func TestMCPHandler_SelectableColumns_PartialFieldsAndErrors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner).Times(3)
	showScanner.EXPECT().Err().Return(nil).Times(3)
	showScanner.EXPECT().Bytes().Return([]byte(`[
		{"column_name":"id","data_type":"integer","position":1},
		{"column_name":"name","data_type":"text","position":2}
	]`)).Times(3)

	h := NewMCPHandler(Deps{Executor: executor, PGDatabase: "prest-test"})
	columns, err := h.selectableColumns(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "prest-test", "public", "users")
	require.NoError(t, err)
	require.Len(t, columns, 2)

	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "alice").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "alice").Return([]string{"name"}, nil)
	h.perms = perms
	req := httptest.NewRequest(http.MethodGet, "/_mcp", nil)
	req = req.WithContext(withUser(req.Context(), auth.User{Username: "alice"}))
	columns, err = h.selectableColumns(req, "prest-test", "public", "users")
	require.NoError(t, err)
	require.Len(t, columns, 1)
	require.Equal(t, "name", columns[0].Name)

	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return(nil, errors.New("fields failed"))
	columns, err = h.selectableColumns(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "prest-test", "public", "users")
	require.Error(t, err)
}

func TestMCPHandler_FilterAccessibleTables_SelectableError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	perms := mockgen.NewMockPermissionsChecker(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true)
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return(nil, errors.New("fields failed"))

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, PGDatabase: "prest-test"})
	rows := []map[string]any{{"schema": "public", "name": "users", "type": "table"}}
	_, err := h.filterAccessibleTables(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "prest-test", rows)
	require.Error(t, err)
}

func TestMCPHandler_RawTableRows_QueryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)

	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), "SELECT table  ").Return(scanner)
	scanner.EXPECT().Err().Return(errors.New("query failed"))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, PGDatabase: "prest-test"})
	_, err := h.rawTableRows(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "prest-test", "")
	require.Error(t, err)
}

func TestMCPHandler_Tools_SkipsInvalidRowsAndDuplicates(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[
		{"schema":"","name":"users","type":"table"},
		{"schema":"public","name":"users","type":"sequence"},
		{"schema":"public","name":"users","type":"table"},
		{"schema":"public","name":"users","type":"table"}
	]`))

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner).AnyTimes()
	showScanner.EXPECT().Err().Return(nil).AnyTimes()
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`)).AnyTimes()
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(true).AnyTimes()
	perms.EXPECT().FieldsPermissions(gomock.Any(), "prest-test", "public", "users", "read", "").Return([]string{"id"}, nil).AnyTimes()

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	tools, err := h.tools(httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.NoError(t, err)

	selectTools := 0
	for _, tool := range tools {
		if tool.Name == "prest.select.prest-test.public.users" {
			selectTools++
		}
	}
	require.Equal(t, 1, selectTools)
}

func TestMCPHandler_Tools_ContinuesOnRawTableError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(errors.New("catalog failed"))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})
	tools, err := h.tools(httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.NoError(t, err)
	require.Len(t, tools, 5)
}

func TestMCPHandler_CallTool_InvalidArgumentsForEachTool(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodPost, "/_mcp", nil)

	_, err := h.callTool(req, json.RawMessage(`{"name":"prest.list_tables","arguments":"bad"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid list tables arguments")

	_, err = h.callTool(req, json.RawMessage(`{"name":"prest.describe_table","arguments":"bad"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid describe arguments")

	_, err = h.callTool(req, json.RawMessage(`{"name":"prest.select_table","arguments":"bad"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid select arguments")
}

func TestMCPHandler_CallTool_InvalidSelectToolNameAndOverride(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{PGDatabase: "prest-test"})
	req := httptest.NewRequest(http.MethodPost, "/_mcp", nil)

	_, err := h.callTool(req, json.RawMessage(`{"name":"prest.select.bad.name.only.two.parts"}`))
	require.Error(t, err)

	_, err = h.callTool(req, json.RawMessage(`{"name":"prest.select.prest-test.public.users","arguments":"bad"}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid select arguments")
}

func TestMCPHandler_CallTool_InvalidToolCallParams(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	_, err := h.callTool(httptest.NewRequest(http.MethodPost, "/_mcp", nil), json.RawMessage(`not-json`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid tool call arguments")
}

func TestMCP_ColumnNames(t *testing.T) {
	t.Parallel()

	names := columnNames([]map[string]any{
		{"column_name": "id"},
		{"name": "email"},
		{"ignored": "x"},
	})
	require.Equal(t, []string{"id", "email"}, names)
}

func TestMCP_ColumnsFromRows(t *testing.T) {
	t.Parallel()

	columns := columnsFromRows([]map[string]any{
		{"column_name": "", "position": 1},
		{"column_name": "id", "data_type": "integer", "is_nullable": "YES", "position": json.Number("2")},
		{"name": "created_at", "data_type": "timestamp with time zone", "position": 1},
	})
	require.Len(t, columns, 2)
	require.Equal(t, "created_at", columns[0].Name)
	require.True(t, columns[1].Nullable)
}

func TestMCP_FirstString(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"n": json.Number("42"),
		"s": testStringer{value: "from-stringer"},
	}
	require.Equal(t, "42", firstString(row, "n"))
	require.Equal(t, "from-stringer", firstString(row, "s"))
	require.Empty(t, firstString(row, "missing"))
}

func TestMCP_YesNoBool(t *testing.T) {
	t.Parallel()

	require.True(t, yesNoBool("YES"))
	require.True(t, yesNoBool("always"))
	require.False(t, yesNoBool("NO"))
}

func TestMCP_IntValue(t *testing.T) {
	t.Parallel()

	require.Equal(t, 7, intValue(int(7)))
	require.Equal(t, 8, intValue(int32(8)))
	require.Equal(t, 9, intValue(int64(9)))
	require.Equal(t, 10, intValue(float64(10)))
	require.Equal(t, 11, intValue(json.Number("11")))
	require.Equal(t, 12, intValue("12"))
	require.Equal(t, 0, intValue(struct{}{}))
}

func TestMCP_ValueSchemaForColumn(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"integer":                  "integer",
		"numeric":                  "number",
		"boolean":                  "boolean",
		"jsonb":                    "object",
		"date":                     "date",
		"timestamp with time zone": "date-time",
		"time without time zone":   "time",
		"text":                     "string",
	}
	for dataType, format := range cases {
		schema := valueSchemaForColumn(mcpColumn{DataType: dataType})
		if format == "date" || format == "date-time" || format == "time" {
			require.Equal(t, "string", schema["type"])
			require.Equal(t, format, schema["format"])
			continue
		}
		require.Equal(t, format, schema["type"])
	}
}

func TestMCP_DescribeToolDescription(t *testing.T) {
	t.Parallel()

	ref := mcpTableRef{Database: "db", Schema: "public", Table: "users"}
	require.Contains(t, describeToolDescription(ref, nil), "db.public.users")

	desc := describeToolDescription(ref, []mcpColumn{
		{Name: "id", DataType: "integer"},
		{Name: "bio"},
	})
	require.Contains(t, desc, "id (integer)")
	require.Contains(t, desc, "bio")
}

func TestMCP_TableRefFromRow(t *testing.T) {
	t.Parallel()

	ref, ok := tableRefFromRow("prest-test", map[string]any{"schema": "public", "name": "users", "type": "view"})
	require.True(t, ok)
	require.Equal(t, "prest-test", ref.Database)
	require.Equal(t, "view", ref.Type)

	_, ok = tableRefFromRow("prest-test", map[string]any{"schema": "public"})
	require.False(t, ok)

	ref, ok = tableRefFromRow("prest-test", map[string]any{
		"schema": "public", "name": "users", "database": "other-db", "type": "table",
	})
	require.True(t, ok)
	require.Equal(t, "other-db", ref.Database)
}

func TestMCP_IsQueryableTableType(t *testing.T) {
	t.Parallel()

	require.True(t, isQueryableTableType(""))
	require.True(t, isQueryableTableType("materialized_view"))
	require.False(t, isQueryableTableType("sequence"))
}

func TestMCP_UniqueStrings(t *testing.T) {
	t.Parallel()

	require.Nil(t, uniqueStrings(nil))
	require.Equal(t, []string{"a", "b"}, uniqueStrings([]string{"", "a", "a", "b"}))
}

func TestMCP_ParseSelectToolName_InvalidSegments(t *testing.T) {
	t.Parallel()

	_, err := parseSelectToolName("prest.select.bad;schema.public.users")
	require.Error(t, err)
}

func TestMCP_DecodeJSONRows_EmptyAndInvalid(t *testing.T) {
	t.Parallel()

	rows, err := decodeJSONRows(nil)
	require.NoError(t, err)
	require.Empty(t, rows)

	_, err = decodeJSONRows([]byte(`not-json`))
	require.Error(t, err)
}

func TestMCP_DecodeWithNumbers_EmptyAndNull(t *testing.T) {
	t.Parallel()

	var out map[string]any
	require.NoError(t, decodeWithNumbers(nil, &out))
	require.NoError(t, decodeWithNumbers([]byte(`null`), &out))
}

type testStringer struct {
	value string
}

func (s testStringer) String() string { return s.value }

func TestMCPHandler_DispatchRPC_ToolsListError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(errors.New("catalog failed"))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})
	_, err := h.dispatchRPC(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "tools/list", nil)
	require.NoError(t, err)
}

func TestMCPHandler_AccessibleSchemas_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)

	catalog.EXPECT().TableClause().Return("SELECT table")
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(errors.New("tables failed"))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, PGDatabase: "prest-test"})
	_, err := h.accessibleSchemas(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "prest-test")
	require.Error(t, err)
}

func TestMCPHandler_ListDatabasesViaCatalog_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)

	catalog.EXPECT().DatabaseClause(gomock.Any()).Return("SELECT datname", true)
	catalog.EXPECT().DatabaseWhere("").Return("")
	catalog.EXPECT().DatabaseOrderBy("", true).Return("")

	scanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(scanner)
	scanner.EXPECT().Err().Return(errors.New("db query failed"))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor})
	_, err := h.listDatabases(httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.Error(t, err)
}

func TestMCPHandler_SelectTable_ValidateTargetError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewMCPHandler(Deps{DB: mockDatabaseRegistry(ctrl), PGDatabase: "prest-test"})
	_, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpSelectArgs{
		Database: "prest-test", Schema: "bad;schema", Table: "users",
	})
	require.Error(t, err)
}

func TestMCPHandler_DescribeTable_ValidateTargetError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewMCPHandler(Deps{DB: mockDatabaseRegistry(ctrl), PGDatabase: "prest-test"})
	_, err := h.describeTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpDescribeArgs{
		Database: "prest-test", Schema: "bad;schema", Table: "users",
	})
	require.Error(t, err)
}

func TestMCPHandler_HandleRPC_DispatchError(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"unknown/method"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported method")
}

func TestMCPHandler_McpTableSelectSchema(t *testing.T) {
	t.Parallel()

	schema := mcpTableSelectSchema([]mcpColumn{
		{Name: "id", DataType: "integer"},
		{Name: "name", DataType: "text"},
	})
	props := schema["properties"].(map[string]any)
	filters := props["filters"].(map[string]any)
	filterProps := filters["properties"].(map[string]any)
	require.Contains(t, filterProps, "id")
	require.Contains(t, filterProps, "name")
}

func TestMCPHandler_PermissionRequestStripsQuery(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/_mcp?token=secret", nil)
	cloned := permissionRequest(req)
	require.Empty(t, cloned.URL.RawQuery)
	require.NotSame(t, req.URL, cloned.URL)
}

func TestMCPHandler_ContainsString(t *testing.T) {
	t.Parallel()

	require.True(t, containsString([]string{"a", "b"}, "b"))
	require.False(t, containsString([]string{"a"}, "c"))
}

func TestMCPHandler_QuotePathSegmentEscapesQuotes(t *testing.T) {
	t.Parallel()

	require.Equal(t, `"a""b"`, quotePathSegment(`a"b`))
}

func TestMCPHandler_ListSchemas_UnregisteredDatabase(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("missing").Return(false)

	h := NewMCPHandler(Deps{DB: db, PGDatabase: "prest-test"})
	_, err := h.listSchemas(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpListSchemasArgs{Database: "missing"})
	require.Error(t, err)
}

func TestMCPHandler_SelectableColumns_TableDenied(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(false)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, PGDatabase: "prest-test"})
	columns, err := h.selectableColumns(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "prest-test", "public", "users")
	require.NoError(t, err)
	require.Nil(t, columns)
}

func TestMCPHandler_Tools_SkipsWhenNoSelectableColumns(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return("SELECT table")
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
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(false)

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	tools, err := h.tools(httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.NoError(t, err)
	for _, tool := range tools {
		require.NotEqual(t, "prest.select.prest-test.public.users", tool.Name)
	}
}

func TestMCPHandler_RPC_SelectTableAccessDenied(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	perms := mockgen.NewMockPermissionsChecker(ctrl)
	db := mockDatabaseRegistry(ctrl)

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id","data_type":"integer","position":1}]`))
	perms.EXPECT().TablePermissions("prest-test", "public", "users", "read", "").Return(false)

	h := NewMCPHandler(Deps{Executor: executor, Perms: perms, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.select_table","arguments":{"database":"prest-test","schema":"public","table":"users"}}}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMCPHandler_ContextWithTimeout(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/_mcp", nil)
	req = req.WithContext(context.WithValue(req.Context(), pctx.HTTPTimeoutKey, 30))
	ctx, cancel := requestContext(req, "prest-test")
	defer cancel()
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.False(t, deadline.IsZero())
}
