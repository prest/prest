package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/adapters"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"
	"golang.org/x/sync/errgroup"
)

const (
	mcpProtocolVersion = "0.1"
	mcpServerName      = "prest"
	mcpSelectPrefix    = "prest.select."
	mcpMaxRows         = 100
)

// MCPHandler serves a read-only MCP-style HTTP endpoint.
type MCPHandler struct {
	catalog  adapters.CatalogQuerier
	builder  adapters.RequestQueryBuilder
	executor adapters.QueryExecutor
	db       adapters.DatabaseRegistry
	perms    adapters.PermissionsChecker
	singleDB bool
	pgDB     string
}

// NewMCPHandler creates an MCPHandler.
func NewMCPHandler(deps Deps) *MCPHandler {
	return &MCPHandler{
		catalog:  deps.Catalog,
		builder:  deps.Builder,
		executor: deps.Executor,
		db:       deps.DB,
		perms:    deps.Perms,
		singleDB: deps.SingleDB,
		pgDB:     deps.PGDatabase,
	}
}

// Handler returns an http.HandlerFunc for route registration.
func (h *MCPHandler) Handler() http.HandlerFunc {
	return h.ServeHTTP
}

// ServeHTTP returns discovery data on GET and handles JSON-RPC style calls on POST.
func (h *MCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.writeJSON(w, http.StatusOK, h.discoveryPayload(r))
	case http.MethodPost:
		h.handleRPC(w, r)
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type mcpDiscoveryPayload struct {
	Name         string    `json:"name"`
	Protocol     string    `json:"protocol"`
	Endpoint     string    `json:"endpoint"`
	Description  string    `json:"description"`
	Tools        []mcpTool `json:"tools"`
	Capabilities any       `json:"capabilities,omitempty"`
}

type mcpJSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpSelectArgs struct {
	Database string         `json:"database"`
	Schema   string         `json:"schema"`
	Table    string         `json:"table"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
	Columns  []string       `json:"columns"`
	OrderBy  []string       `json:"order_by"`
	Filters  map[string]any `json:"filters"`
}

type mcpDescribeArgs struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
}

type mcpListSchemasArgs struct {
	Database string `json:"database"`
}

type mcpListTablesArgs struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
}

type mcpColumn struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type,omitempty"`
	Nullable     bool   `json:"nullable"`
	MaxLength    string `json:"max_length,omitempty"`
	Generated    bool   `json:"generated,omitempty"`
	Updatable    bool   `json:"updatable,omitempty"`
	DefaultValue string `json:"default_value,omitempty"`
	Position     int    `json:"position,omitempty"`
}

type mcpTableRef struct {
	Database string
	Schema   string
	Table    string
	Type     string
}

type mcpSelectResult struct {
	Database string           `json:"database"`
	Schema   string           `json:"schema"`
	Table    string           `json:"table"`
	Columns  []string         `json:"columns,omitempty"`
	Rows     []map[string]any `json:"rows"`
	Count    int              `json:"count"`
}

func (h *MCPHandler) handleRPC(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req mcpJSONRPCRequest
	if err := dec.Decode(&req); err != nil {
		h.writeRPCError(w, nil, http.StatusBadRequest, "invalid request", err.Error())
		return
	}
	if req.Method == "" {
		h.writeRPCError(w, req.ID, http.StatusBadRequest, "invalid request", "missing method")
		return
	}
	// JSON-RPC notification (no id): accept with empty body; do not dispatch a response.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	result, err := h.dispatchRPC(r, req.Method, req.Params)
	if err != nil {
		h.writeRPCError(w, req.ID, http.StatusBadRequest, err.Error(), nil)
		return
	}

	h.writeJSON(w, http.StatusOK, mcpJSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

func (h *MCPHandler) dispatchRPC(r *http.Request, method string, params json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		return map[string]any{
			"serverInfo": map[string]any{"name": mcpServerName, "version": mcpProtocolVersion},
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"instructions": "Read-only tools are exposed through pREST auth and ACL.",
		}, nil
	case "tools/list":
		tools, err := h.tools(r)
		if err != nil {
			return nil, err
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		return h.callTool(r, params)
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

func (h *MCPHandler) discoveryPayload(r *http.Request) mcpDiscoveryPayload {
	tools, err := h.tools(r)
	if err != nil {
		return mcpDiscoveryPayload{
			Name:        mcpServerName,
			Protocol:    mcpProtocolVersion,
			Endpoint:    r.URL.Path,
			Description: err.Error(),
		}
	}

	return mcpDiscoveryPayload{
		Name:        mcpServerName,
		Protocol:    mcpProtocolVersion,
		Endpoint:    r.URL.Path,
		Description: "Read-only MCP endpoint backed by pREST catalog and query execution.",
		Tools:       tools,
		Capabilities: map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
	}
}

func (h *MCPHandler) callTool(r *http.Request, params json.RawMessage) (any, error) {
	var payload struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := decodeWithNumbers(params, &payload); err != nil {
		return nil, fmt.Errorf("invalid tool call arguments: %w", err)
	}
	if payload.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	switch payload.Name {
	case "prest.list_databases":
		return h.listDatabases(r)
	case "prest.list_schemas":
		var args mcpListSchemasArgs
		if err := decodeWithNumbers(payload.Arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid list schemas arguments: %w", err)
		}
		return h.listSchemas(r, args)
	case "prest.list_tables":
		var args mcpListTablesArgs
		if err := decodeWithNumbers(payload.Arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid list tables arguments: %w", err)
		}
		return h.listTables(r, args)
	case "prest.describe_table":
		var args mcpDescribeArgs
		if err := decodeWithNumbers(payload.Arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid describe arguments: %w", err)
		}
		return h.describeTable(r, args)
	case "prest.select_table":
		var args mcpSelectArgs
		if err := decodeWithNumbers(payload.Arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid select arguments: %w", err)
		}
		return h.selectTable(r, args)
	default:
		if !strings.HasPrefix(payload.Name, mcpSelectPrefix) {
			return nil, fmt.Errorf("unsupported tool: %s", payload.Name)
		}
		args, err := parseSelectToolName(payload.Name)
		if err != nil {
			return nil, err
		}
		if len(payload.Arguments) > 0 && string(payload.Arguments) != "null" {
			var override mcpSelectArgs
			if err := decodeWithNumbers(payload.Arguments, &override); err != nil {
				return nil, fmt.Errorf("invalid select arguments: %w", err)
			}
			override.Database = args.Database
			override.Schema = args.Schema
			override.Table = args.Table
			if override.Limit == 0 {
				override.Limit = args.Limit
			}
			args = override
		}
		return h.selectTable(r, args)
	}
}

func (h *MCPHandler) listDatabases(r *http.Request) (any, error) {
	aliases := h.databaseAliases()
	if len(aliases) > 0 {
		rows := make([]map[string]any, 0, len(aliases))
		for _, alias := range aliases {
			rows = append(rows, map[string]any{
				"name":          alias,
				"datname":       alias,
				"physical_name": h.physicalDatabase(alias),
			})
		}
		return rows, nil
	}

	query, hasCount := h.catalog.DatabaseClause(httptest.NewRequest(http.MethodGet, "/databases", nil))
	query = fmt.Sprint(query, " ", h.catalog.DatabaseWhere(""), " ", h.catalog.DatabaseOrderBy("", hasCount))
	return h.queryRows(r, query, h.defaultDatabase())
}

func (h *MCPHandler) listSchemas(r *http.Request, args mcpListSchemasArgs) (any, error) {
	if args.Database == "" {
		args.Database = h.defaultDatabase()
	}
	if err := validateDatabase(args.Database, h.db, h.singleDB); err != nil {
		return nil, err
	}

	query, hasCount := h.catalog.SchemaClause(httptest.NewRequest(http.MethodGet, "/schemas", nil))
	query = fmt.Sprint(query, " ", h.catalog.SchemaOrderBy("", hasCount))
	result, err := h.queryRows(r, query, args.Database)
	if err != nil {
		return nil, err
	}
	rows, ok := result.([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected schema discovery result")
	}
	schemas, err := h.accessibleSchemas(r, args.Database)
	if err != nil {
		return nil, err
	}

	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		schema := firstString(row, "schema", "schema_name", "nspname")
		if schema == "" {
			continue
		}
		if h.perms == nil || schemas[schema] {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func (h *MCPHandler) listTables(r *http.Request, args mcpListTablesArgs) (any, error) {
	if args.Database == "" {
		args.Database = h.defaultDatabase()
	}
	if err := validateDatabase(args.Database, h.db, h.singleDB); err != nil {
		return nil, err
	}
	if args.Schema != "" && !validatePathSegments(args.Schema) {
		return nil, fmt.Errorf("invalid identifier in path")
	}
	return h.tableRows(r, args.Database, args.Schema)
}

func (h *MCPHandler) describeTable(r *http.Request, args mcpDescribeArgs) (any, error) {
	if args.Database == "" {
		args.Database = h.defaultDatabase()
	}
	if err := h.validateToolTarget(args.Database, args.Schema, args.Table); err != nil {
		return nil, err
	}

	columns, err := h.describeColumns(r, args.Database, args.Schema, args.Table)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"database": args.Database,
		"schema":   args.Schema,
		"table":    args.Table,
		"columns":  columns,
		"count":    len(columns),
	}, nil
}

func (h *MCPHandler) selectTable(r *http.Request, args mcpSelectArgs) (any, error) {
	if args.Database == "" {
		args.Database = h.defaultDatabase()
	}
	if args.Limit <= 0 || args.Limit > mcpMaxRows {
		args.Limit = mcpMaxRows
	}
	if args.Offset < 0 {
		args.Offset = 0
	}
	if err := h.validateToolTarget(args.Database, args.Schema, args.Table); err != nil {
		return nil, err
	}

	columns, err := h.selectableColumns(r, args.Database, args.Schema, args.Table)
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("you don't have permission for this action, please check the permitted fields for this table")
	}

	columnSet := make(map[string]mcpColumn, len(columns))
	selectedColumns := make([]string, 0, len(columns))
	for _, col := range columns {
		columnSet[col.Name] = col
		selectedColumns = append(selectedColumns, col.Name)
	}
	if len(args.Columns) > 0 {
		selectedColumns = nil
		seen := make(map[string]struct{}, len(args.Columns))
		for _, name := range args.Columns {
			if _, ok := columnSet[name]; !ok {
				return nil, fmt.Errorf("unsupported column: %s", name)
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			selectedColumns = append(selectedColumns, name)
		}
	}

	quotedColumns := make([]string, 0, len(selectedColumns))
	for _, name := range selectedColumns {
		quotedColumns = append(quotedColumns, quotePathSegment(name))
	}

	query := fmt.Sprintf("SELECT %s FROM %s.%s", strings.Join(quotedColumns, ", "), quotePathSegment(args.Schema), quotePathSegment(args.Table))
	whereClause, values, err := buildFilterClause(args.Filters, columnSet)
	if err != nil {
		return nil, err
	}
	if whereClause != "" {
		query = fmt.Sprintf("%s WHERE %s", query, whereClause)
	}
	orderClause, err := buildOrderClause(args.OrderBy, columnSet)
	if err != nil {
		return nil, err
	}
	if orderClause != "" {
		query = fmt.Sprintf("%s %s", query, orderClause)
	}
	query = fmt.Sprintf("%s LIMIT %d OFFSET %d", query, args.Limit, args.Offset)

	ctx, cancel := requestContext(r, args.Database)
	defer cancel()

	sc := h.executor.QueryCtx(ctx, query, values...)
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("select table failed: %w", err)
	}

	rows, err := decodeJSONRows(sc.Bytes())
	if err != nil {
		return nil, err
	}

	return mcpSelectResult{
		Database: args.Database,
		Schema:   args.Schema,
		Table:    args.Table,
		Columns:  selectedColumns,
		Rows:     rows,
		Count:    len(rows),
	}, nil
}

func (h *MCPHandler) tools(r *http.Request) ([]mcpTool, error) {
	tools := []mcpTool{
		{Name: "prest.list_databases", Description: "List accessible databases.", InputSchema: emptyObjectSchema()},
		{Name: "prest.list_schemas", Description: "List readable schemas for a database alias.", InputSchema: mcpListSchemasSchema()},
		{Name: "prest.list_tables", Description: "List readable tables for a database alias and optional schema.", InputSchema: mcpListTablesSchema()},
		{Name: "prest.describe_table", Description: "Describe a table and return its columns.", InputSchema: mcpDescribeSchema()},
		{Name: "prest.select_table", Description: "Read rows from a table in a read-only way.", InputSchema: mcpSelectSchema()},
	}

	aliases := h.databaseAliases()
	if len(aliases) == 0 {
		return tools, nil
	}

	seen := make(map[string]struct{})
	for _, database := range aliases {
		var (
			rows           []map[string]any
			columnsByTable map[string][]mcpColumn
		)
		var group errgroup.Group
		group.Go(func() error {
			var err error
			rows, err = h.rawTableRows(r, database, "")
			return err
		})
		group.Go(func() error {
			var err error
			columnsByTable, err = h.columnsByTable(r, database)
			return err
		})
		if err := group.Wait(); err != nil || len(rows) == 0 {
			continue
		}
		for _, row := range rows {
			ref, ok := tableRefFromRow(database, row)
			if !ok || !isQueryableTableType(ref.Type) {
				continue
			}
			columns, err := h.filterColumnsByPermissions(r, ref.Database, ref.Schema, ref.Table, columnsByTable[tableColumnsKey(ref.Schema, ref.Table)])
			if err != nil || len(columns) == 0 {
				continue
			}
			name := strings.Join([]string{strings.TrimSuffix(mcpSelectPrefix, "."), ref.Database, ref.Schema, ref.Table}, ".")
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			tools = append(tools, mcpTool{
				Name:        name,
				Description: describeToolDescription(ref, columns),
				InputSchema: mcpTableSelectSchema(columns),
			})
		}
	}

	sort.SliceStable(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

func (h *MCPHandler) columnsByTable(r *http.Request, database string) (map[string][]mcpColumn, error) {
	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.ShowColumnsCtx(ctx)
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("list columns failed: %w", err)
	}
	rows, err := decodeJSONRows(sc.Bytes())
	if err != nil {
		return nil, err
	}

	groupedRows := make(map[string][]map[string]any)
	for _, row := range rows {
		schema := firstString(row, "table_schema", "schema")
		table := firstString(row, "table_name", "name")
		if schema == "" || table == "" {
			continue
		}
		key := tableColumnsKey(schema, table)
		groupedRows[key] = append(groupedRows[key], row)
	}

	columns := make(map[string][]mcpColumn, len(groupedRows))
	for key, rows := range groupedRows {
		columns[key] = columnsFromRows(rows)
	}
	return columns, nil
}

func tableColumnsKey(schema, table string) string {
	return schema + "." + table
}

func (h *MCPHandler) describeColumns(r *http.Request, database, schema, table string) ([]mcpColumn, error) {
	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.ShowTableCtx(ctx, schema, table)
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("describe table failed: %w", err)
	}

	rows, err := decodeJSONRows(sc.Bytes())
	if err != nil {
		return nil, err
	}
	return columnsFromRows(rows), nil
}

func (h *MCPHandler) tableRows(r *http.Request, database, schema string) ([]map[string]any, error) {
	rows, err := h.rawTableRows(r, database, schema)
	if err != nil {
		return nil, err
	}
	return h.filterAccessibleTables(r, database, rows)
}

func (h *MCPHandler) rawTableRows(r *http.Request, database, schema string) ([]map[string]any, error) {
	var (
		query  string
		values []interface{}
	)
	if schema != "" {
		query = fmt.Sprint(h.catalog.SchemaTablesClause(), h.catalog.SchemaTablesWhere(""), h.catalog.SchemaTablesOrderBy(""))
		values = []interface{}{database, schema}
	} else {
		query = fmt.Sprint(h.catalog.TableClause(), " ", h.catalog.TableWhere(""), " ", h.catalog.TableOrderBy(""))
	}
	result, err := h.queryRows(r, query, database, values...)
	if err != nil {
		return nil, err
	}
	rows, ok := result.([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected table discovery result")
	}
	return rows, nil
}

func (h *MCPHandler) queryRows(r *http.Request, query string, database string, values ...interface{}) (any, error) {
	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.QueryCtx(ctx, query, values...)
	if err := sc.Err(); err != nil {
		return nil, err
	}
	rows, err := decodeJSONRows(sc.Bytes())
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (h *MCPHandler) filterAccessibleTables(r *http.Request, database string, rows []map[string]any) ([]map[string]any, error) {
	if len(rows) == 0 {
		return []map[string]any{}, nil
	}

	var columnsByTable map[string][]mcpColumn
	if h.perms != nil {
		var err error
		columnsByTable, err = h.columnsByTable(r, database)
		if err != nil {
			return nil, err
		}
	}

	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		ref, ok := tableRefFromRow(database, row)
		if !ok || !isQueryableTableType(ref.Type) {
			continue
		}
		if h.perms != nil {
			columns, err := h.filterColumnsByPermissions(r, ref.Database, ref.Schema, ref.Table, columnsByTable[tableColumnsKey(ref.Schema, ref.Table)])
			if err != nil {
				return nil, err
			}
			if len(columns) == 0 {
				continue
			}
		}
		filtered = append(filtered, row)
	}
	return filtered, nil
}

func (h *MCPHandler) accessibleSchemas(r *http.Request, database string) (map[string]bool, error) {
	tables, err := h.tableRows(r, database, "")
	if err != nil {
		return nil, err
	}
	schemas := make(map[string]bool, len(tables))
	for _, row := range tables {
		schema := firstString(row, "schema", "schema_name", "nspname")
		if schema != "" {
			schemas[schema] = true
		}
	}
	return schemas, nil
}

func (h *MCPHandler) selectableColumns(r *http.Request, database, schema, table string) ([]mcpColumn, error) {
	columns, err := h.describeColumns(r, database, schema, table)
	if err != nil {
		return nil, err
	}
	return h.filterColumnsByPermissions(r, database, schema, table, columns)
}

func (h *MCPHandler) filterColumnsByPermissions(r *http.Request, database, schema, table string, columns []mcpColumn) ([]mcpColumn, error) {
	if h.perms == nil {
		return columns, nil
	}

	userName := currentUserName(r)
	if !h.perms.TablePermissions(database, schema, table, "read", userName) {
		return nil, nil
	}
	fields, err := h.perms.FieldsPermissions(permissionRequest(r), database, schema, table, "read", userName)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, nil
	}
	if containsString(fields, "*") {
		return columns, nil
	}
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[field] = true
	}
	filtered := make([]mcpColumn, 0, len(columns))
	for _, col := range columns {
		if allowed[col.Name] {
			filtered = append(filtered, col)
		}
	}
	return filtered, nil
}

func (h *MCPHandler) validateToolTarget(database, schema, table string) error {
	if database == "" {
		database = h.defaultDatabase()
	}
	if err := validateDatabase(database, h.db, h.singleDB); err != nil {
		return err
	}
	if !validatePathSegments(database, schema, table) {
		return fmt.Errorf("invalid identifier in path")
	}
	return nil
}

func (h *MCPHandler) physicalDatabase(alias string) string {
	if h.db == nil {
		return alias
	}
	return h.db.PhysicalName(alias)
}

func (h *MCPHandler) databaseAliases() []string {
	if h.singleDB {
		database := h.defaultDatabase()
		if database == "" {
			return nil
		}
		return []string{database}
	}
	if h.db != nil {
		aliases := uniqueStrings(h.db.Aliases())
		if len(aliases) > 0 {
			return aliases
		}
	}
	database := h.defaultDatabase()
	if database == "" {
		return nil
	}
	return []string{database}
}

func (h *MCPHandler) defaultDatabase() string {
	if h.pgDB != "" {
		return h.pgDB
	}
	if h.db != nil {
		return h.db.GetDatabase()
	}
	return ""
}

func (h *MCPHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *MCPHandler) writeRPCError(w http.ResponseWriter, id json.RawMessage, status int, message string, data any) {
	h.writeJSON(w, status, mcpJSONRPCResponse{JSONRPC: "2.0", ID: id, Error: &mcpError{Code: status, Message: message, Data: data}})
}

func decodeJSONRows(raw []byte) ([]map[string]any, error) {
	if len(raw) == 0 {
		return []map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var rows []map[string]any
	if err := dec.Decode(&rows); err != nil {
		return nil, fmt.Errorf("decode query result: %w", err)
	}
	return rows, nil
}

func decodeWithNumbers(raw []byte, dst any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(dst)
}

func columnNames(rows []map[string]any) []string {
	cols := make([]string, 0, len(rows))
	for _, row := range rows {
		name := firstString(row, "column_name", "name", "field")
		if name != "" {
			cols = append(cols, name)
		}
	}
	return cols
}

func columnsFromRows(rows []map[string]any) []mcpColumn {
	columns := make([]mcpColumn, 0, len(rows))
	for _, row := range rows {
		name := firstString(row, "column_name", "name", "field")
		if name == "" {
			continue
		}
		columns = append(columns, mcpColumn{
			Name:         name,
			DataType:     firstString(row, "data_type"),
			Nullable:     yesNoBool(firstString(row, "is_nullable")),
			MaxLength:    firstString(row, "max_length"),
			Generated:    yesNoBool(firstString(row, "is_generated")),
			Updatable:    yesNoBool(firstString(row, "is_updatable")),
			DefaultValue: firstString(row, "default_value"),
			Position:     intValue(row["position"]),
		})
	}
	sort.SliceStable(columns, func(i, j int) bool { return columns[i].Position < columns[j].Position })
	return columns
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			switch v := value.(type) {
			case string:
				return v
			case json.Number:
				return v.String()
			case fmt.Stringer:
				return v.String()
			}
		}
	}
	return ""
}

func yesNoBool(value string) bool {
	switch strings.ToUpper(value) {
	case "YES", "ALWAYS", "TRUE":
		return true
	default:
		return false
	}
}

func intValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

func quotePathSegment(segment string) string {
	return `"` + strings.ReplaceAll(segment, `"`, `""`) + `"`
}

func emptyObjectSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func mcpListSchemasSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"database": map[string]any{"type": "string"},
		},
	}
}

func mcpListTablesSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"database": map[string]any{"type": "string"},
			"schema":   map[string]any{"type": "string"},
		},
	}
}

func mcpDescribeSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"database": map[string]any{"type": "string"},
			"schema":   map[string]any{"type": "string"},
			"table":    map[string]any{"type": "string"},
		},
		"required": []string{"database", "schema", "table"},
	}
}

func mcpSelectSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"database": map[string]any{"type": "string"},
			"schema":   map[string]any{"type": "string"},
			"table":    map[string]any{"type": "string"},
			"limit":    map[string]any{"type": "integer", "minimum": 1, "maximum": mcpMaxRows},
			"offset":   map[string]any{"type": "integer", "minimum": 0},
			"columns":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "uniqueItems": true},
			"order_by": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "uniqueItems": true},
			"filters":  genericFilterSchema(),
		},
		"required": []string{"database", "schema", "table"},
	}
}

func genericFilterSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"additionalProperties": map[string]any{
			"anyOf": []any{
				map[string]any{"type": "string"},
				map[string]any{"type": "number"},
				map[string]any{"type": "integer"},
				map[string]any{"type": "boolean"},
				map[string]any{"type": "null"},
				map[string]any{
					"type": "array",
					"items": map[string]any{
						"anyOf": []any{
							map[string]any{"type": "string"},
							map[string]any{"type": "number"},
							map[string]any{"type": "integer"},
							map[string]any{"type": "boolean"},
						},
					},
				},
			},
		},
	}
}

func mcpTableSelectSchema(columns []mcpColumn) map[string]any {
	columnNames := make([]string, 0, len(columns))
	orderValues := make([]string, 0, len(columns)*2)
	filterProperties := make(map[string]any, len(columns))
	for _, col := range columns {
		columnNames = append(columnNames, col.Name)
		orderValues = append(orderValues, col.Name, "-"+col.Name)
		filterProperties[col.Name] = nullableSchema(filterSchemaForColumn(col))
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": mcpMaxRows},
			"offset": map[string]any{"type": "integer", "minimum": 0},
			"columns": map[string]any{
				"type":        "array",
				"uniqueItems": true,
				"items":       map[string]any{"type": "string", "enum": columnNames},
			},
			"order_by": map[string]any{
				"type":        "array",
				"uniqueItems": true,
				"items":       map[string]any{"type": "string", "enum": orderValues},
			},
			"filters": map[string]any{
				"type":                 "object",
				"properties":           filterProperties,
				"additionalProperties": false,
			},
		},
	}
}

func filterSchemaForColumn(col mcpColumn) map[string]any {
	base := valueSchemaForColumn(col)
	return map[string]any{
		"anyOf": []any{
			base,
			map[string]any{"type": "array", "items": base},
		},
	}
}

func valueSchemaForColumn(col mcpColumn) map[string]any {
	switch strings.ToLower(col.DataType) {
	case "smallint", "integer", "bigint", "serial", "bigserial":
		return map[string]any{"type": "integer"}
	case "numeric", "decimal", "real", "double precision":
		return map[string]any{"type": "number"}
	case "boolean":
		return map[string]any{"type": "boolean"}
	case "json", "jsonb":
		return map[string]any{"type": "object"}
	case "date":
		return map[string]any{"type": "string", "format": "date"}
	case "timestamp without time zone", "timestamp with time zone":
		return map[string]any{"type": "string", "format": "date-time"}
	case "time without time zone", "time with time zone":
		return map[string]any{"type": "string", "format": "time"}
	default:
		return map[string]any{"type": "string"}
	}
}

func nullableSchema(schema map[string]any) map[string]any {
	return map[string]any{"anyOf": []any{schema, map[string]any{"type": "null"}}}
}

func buildFilterClause(filters map[string]any, columns map[string]mcpColumn) (string, []interface{}, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	clauses := make([]string, 0, len(keys))
	values := make([]interface{}, 0, len(keys))
	index := 1
	for _, key := range keys {
		col, ok := columns[key]
		if !ok {
			return "", nil, fmt.Errorf("unsupported filter column: %s", key)
		}
		quoted := quotePathSegment(col.Name)
		value := filters[key]
		if value == nil {
			clauses = append(clauses, fmt.Sprintf("%s IS NULL", quoted))
			continue
		}
		if arr, ok := value.([]any); ok {
			if len(arr) == 0 {
				return "", nil, fmt.Errorf("filter array cannot be empty: %s", key)
			}
			placeholders := make([]string, 0, len(arr))
			for _, item := range arr {
				placeholders = append(placeholders, fmt.Sprintf("$%d", index))
				values = append(values, item)
				index++
			}
			clauses = append(clauses, fmt.Sprintf("%s IN (%s)", quoted, strings.Join(placeholders, ", ")))
			continue
		}
		clauses = append(clauses, fmt.Sprintf("%s = $%d", quoted, index))
		values = append(values, value)
		index++
	}
	return strings.Join(clauses, " AND "), values, nil
}

func buildOrderClause(orderBy []string, columns map[string]mcpColumn) (string, error) {
	if len(orderBy) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(orderBy))
	for _, item := range orderBy {
		direction := "ASC"
		name := item
		if strings.HasPrefix(name, "-") {
			direction = "DESC"
			name = strings.TrimPrefix(name, "-")
		}
		if _, ok := columns[name]; !ok {
			return "", fmt.Errorf("unsupported order column: %s", name)
		}
		parts = append(parts, fmt.Sprintf("%s %s", quotePathSegment(name), direction))
	}
	return "ORDER BY " + strings.Join(parts, ", "), nil
}

func describeToolDescription(ref mcpTableRef, columns []mcpColumn) string {
	if len(columns) == 0 {
		return fmt.Sprintf("Read rows from %s.%s.%s.", ref.Database, ref.Schema, ref.Table)
	}
	parts := make([]string, 0, len(columns))
	for _, col := range columns {
		if col.DataType == "" {
			parts = append(parts, col.Name)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", col.Name, col.DataType))
	}
	return fmt.Sprintf("Read rows from %s.%s.%s. Readable columns: %s.", ref.Database, ref.Schema, ref.Table, strings.Join(parts, ", "))
}

func tableRefFromRow(database string, row map[string]any) (mcpTableRef, bool) {
	schema := firstString(row, "schema", "schemaname", "schema_name")
	table := firstString(row, "name", "tablename", "table_name")
	if schema == "" || table == "" {
		return mcpTableRef{}, false
	}
	rowDB := firstString(row, "database", "catalog_name")
	if rowDB == "" {
		rowDB = database
	}
	return mcpTableRef{
		Database: rowDB,
		Schema:   schema,
		Table:    table,
		Type:     firstString(row, "type"),
	}, true
}

func isQueryableTableType(kind string) bool {
	if kind == "" {
		return true
	}
	switch kind {
	case "table", "view", "materialized_view", "foreign_table":
		return true
	default:
		return false
	}
}

func permissionRequest(r *http.Request) *http.Request {
	req := r.Clone(r.Context())
	copyURL := *req.URL
	copyURL.RawQuery = ""
	req.URL = &copyURL
	return req
}

func currentUserName(r *http.Request) string {
	userInfo := r.Context().Value(pctx.UserInfoKey)
	if user, ok := userInfo.(auth.User); ok {
		return user.Username
	}
	return ""
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	uniq := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		uniq = append(uniq, item)
	}
	sort.Strings(uniq)
	return uniq
}

func parseSelectToolName(name string) (mcpSelectArgs, error) {
	trimmed := strings.TrimPrefix(name, mcpSelectPrefix)
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return mcpSelectArgs{}, fmt.Errorf("invalid select tool name: %s", name)
	}
	if !validatePathSegments(parts...) {
		return mcpSelectArgs{}, fmt.Errorf("invalid select tool name: %s", name)
	}
	limit := mcpMaxRows
	if limit < 1 {
		limit = mcpMaxRows
	}
	return mcpSelectArgs{Database: parts[0], Schema: parts[1], Table: parts[2], Limit: limit}, nil
}
