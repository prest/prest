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
