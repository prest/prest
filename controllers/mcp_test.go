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
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return(`SELECT n.nspname as "schema", c.relname as "name" FROM pg_catalog.pg_class c`)
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users","type":"table","owner":"postgres"}]`))

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id"},{"column_name":"name"}]`))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/_mcp", nil)

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "prest.select.prest-test.public.users")
	require.Contains(t, rec.Body.String(), "columns")
}

func TestMCPHandler_Handler(t *testing.T) {
	t.Parallel()

	h := NewMCPHandler(Deps{})
	require.NotNil(t, h.Handler())
}

func TestMCPHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	h := NewMCPHandler(Deps{})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/_mcp", nil))

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
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

func TestMCPHandler_DispatchRPC(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return(`SELECT n.nspname as "schema", c.relname as "name" FROM pg_catalog.pg_class c`)
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users"}]`))

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner).Times(2)
	showScanner.EXPECT().Err().Return(nil).AnyTimes()
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id"}]`)).AnyTimes()

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})

	result, err := h.dispatchRPC(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "initialize", nil)
	require.NoError(t, err)
	require.Contains(t, result.(map[string]any), "serverInfo")

	result, err = h.dispatchRPC(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "tools/list", nil)
	require.NoError(t, err)
	require.Contains(t, result.(map[string]any), "tools")

	result, err = h.dispatchRPC(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "tools/call", []byte(`{"name":"prest.describe_table","arguments":{"database":"prest-test","schema":"public","table":"users"}}`))
	require.NoError(t, err)
	require.Contains(t, result.(map[string]any), "columns")

	_, err = h.dispatchRPC(httptest.NewRequest(http.MethodGet, "/_mcp", nil), "unsupported", nil)
	require.Error(t, err)
}

func TestMCPHandler_DiscoveryErrorAndQueryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().TableClause().Return(`SELECT n.nspname as "schema", c.relname as "name" FROM pg_catalog.pg_class c`)
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")
	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner)
	tableScanner.EXPECT().Err().Return(errors.New("query failed"))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "query failed")
}

func TestMCPHandler_ListToolsAndSelectHelpers(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	db.EXPECT().IsRegistered("prest-test").Return(true).AnyTimes()
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	catalog.EXPECT().TableClause().Return(`SELECT n.nspname as "schema", c.relname as "name" FROM pg_catalog.pg_class c`)
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")

	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner).Times(1)
	tableScanner.EXPECT().Err().Return(nil).AnyTimes()
	tableScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users"}]`)).AnyTimes()

	selectScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), `SELECT * FROM "public"."users" LIMIT 100 OFFSET 0`).Return(selectScanner)
	selectScanner.EXPECT().Err().Return(nil)
	selectScanner.EXPECT().Bytes().Return([]byte(`[{"id":1,"name":"Alice"}]`))

	showScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().ShowTableCtx(gomock.Any(), "public", "users").Return(showScanner)
	showScanner.EXPECT().Err().Return(nil)
	showScanner.EXPECT().Bytes().Return([]byte(`[{"column_name":"id"},{"column_name":"name"}]`))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})

	tools, err := h.tools(httptest.NewRequest(http.MethodGet, "/_mcp", nil))
	require.NoError(t, err)
	require.NotEmpty(t, tools)
	require.Contains(t, tools[0].Name, "prest.")

	args, err := parseSelectToolName("prest.select.prest-test.public.users")
	require.NoError(t, err)
	selectResult, err := h.selectTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), args)
	require.NoError(t, err)
	require.Len(t, selectResult.(mcpSelectResult).Rows, 1)
	require.Equal(t, 1, selectResult.(mcpSelectResult).Count)

	_, err = parseSelectToolName("invalid")
	require.Error(t, err)
	require.Equal(t, "prest-test", h.defaultDatabase())
	err = h.validateToolTarget("invalid db", "public", "users")
	require.Error(t, err)
}

func TestMCPHandler_DefaultDatabaseAndHelpers(t *testing.T) {
	t.Parallel()

	require.Equal(t, "prest-test", NewMCPHandler(Deps{PGDatabase: "prest-test"}).defaultDatabase())
	require.Equal(t, "", NewMCPHandler(Deps{}).defaultDatabase())
	require.Equal(t, []map[string]any{}, mustDecodeRows(t, []byte("[]")))
	require.Equal(t, "abc", firstString(map[string]any{"name": "abc"}, "name"))
	require.Equal(t, `"x"`, quotePathSegment("x"))
}

func mustDecodeRows(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	rows, err := decodeJSONRows(raw)
	require.NoError(t, err)
	return rows
}

func TestMCPHandler_ToolsCallSelectTable(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	executor := mockgen.NewMockQueryExecutor(ctrl)
	scanner := mockgen.NewMockScanner(ctrl)
	db := mockDatabaseRegistry(ctrl)

	db.EXPECT().IsRegistered("prest-test").Return(true).AnyTimes()
	db.EXPECT().GetDatabase().Return("prest-test").AnyTimes()

	executor.EXPECT().QueryCtx(gomock.Any(), `SELECT * FROM "public"."users" LIMIT 100 OFFSET 0`).Return(scanner)
	scanner.EXPECT().Err().Return(nil)
	scanner.EXPECT().Bytes().Return([]byte(`[{"id":1,"name":"Alice"}]`))

	h := NewMCPHandler(Deps{Executor: executor, DB: db, PGDatabase: "prest-test"})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.select.prest-test.public.users"}}`)
	req := httptest.NewRequest(http.MethodPost, "/_mcp", body)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"Alice"`)
	require.Contains(t, rec.Body.String(), `"count":1`)
}

func TestMCPHandler_ToolsCallListMethods(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	catalog := mockgen.NewMockCatalogQuerier(ctrl)
	executor := mockgen.NewMockQueryExecutor(ctrl)
	db := mockDatabaseRegistry(ctrl)

	catalog.EXPECT().DatabaseClause(gomock.Any()).Return("SELECT datname FROM pg_database", false)
	catalog.EXPECT().DatabaseWhere("").Return("")
	catalog.EXPECT().DatabaseOrderBy("", false).Return("")
	dbScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(dbScanner)
	dbScanner.EXPECT().Err().Return(nil)
	dbScanner.EXPECT().Bytes().Return([]byte(`[{"datname":"prest-test"}]`))

	catalog.EXPECT().SchemaClause(gomock.Any()).Return("SELECT nspname FROM pg_namespace", false)
	catalog.EXPECT().SchemaOrderBy("", false).Return("")
	schemaScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(schemaScanner)
	schemaScanner.EXPECT().Err().Return(nil)
	schemaScanner.EXPECT().Bytes().Return([]byte(`[{"nspname":"public"}]`))

	catalog.EXPECT().TableClause().Return(`SELECT n.nspname as "schema", c.relname as "name" FROM pg_catalog.pg_class c`)
	catalog.EXPECT().TableWhere("").Return("")
	catalog.EXPECT().TableOrderBy("").Return("")
	tableScanner := mockgen.NewMockScanner(ctrl)
	executor.EXPECT().QueryCtx(gomock.Any(), gomock.Any()).Return(tableScanner)
	tableScanner.EXPECT().Err().Return(nil)
	tableScanner.EXPECT().Bytes().Return([]byte(`[{"schema":"public","name":"users"}]`))

	h := NewMCPHandler(Deps{Catalog: catalog, Executor: executor, DB: db, PGDatabase: "prest-test"})

	for _, tool := range []string{"prest.list_databases", "prest.list_schemas", "prest.list_tables"} {
		body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `"}}`)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/_mcp", body))
		require.Equal(t, http.StatusOK, rec.Code)
	}
}

func TestMCPHandler_DescribeTableInvalidTarget(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	db := mockgen.NewMockDatabaseRegistry(ctrl)
	db.EXPECT().IsRegistered("missing").Return(false)

	h := NewMCPHandler(Deps{DB: db})
	_, err := h.describeTable(httptest.NewRequest(http.MethodGet, "/_mcp", nil), mcpDescribeArgs{Database: "missing", Schema: "public", Table: "users"})
	require.Error(t, err)
}

func TestMCPHandler_WriteRPCError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	NewMCPHandler(Deps{}).writeRPCError(rec, json.RawMessage("1"), http.StatusBadRequest, "boom", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "boom")
}

func TestMCPHandler_ToolsCallRejectsUnknownTool(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	h := NewMCPHandler(Deps{})
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prest.drop_table","arguments":{"sql":"DROP TABLE users"}}}`)
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
}

func TestDecodeJSONRows(t *testing.T) {
	t.Parallel()

	rows, err := decodeJSONRows([]byte(`[{"id":1},{"id":2}]`))
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestMCPHandler_WriteJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	NewMCPHandler(Deps{}).writeJSON(rec, http.StatusOK, map[string]any{"ok": true})

	require.Equal(t, http.StatusOK, rec.Code)
	var decoded map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&decoded))
	require.Equal(t, true, decoded["ok"])
}
