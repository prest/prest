package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"

	"github.com/prest/prest/v2/adapters"
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
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

type mcpDescribeArgs struct {
	Database string `json:"database"`
	Schema   string `json:"schema"`
	Table    string `json:"table"`
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
	if err := json.Unmarshal(params, &payload); err != nil {
		return nil, fmt.Errorf("invalid tool call arguments: %w", err)
	}
	if payload.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	switch payload.Name {
	case "prest.list_databases":
		return h.listDatabases(r)
	case "prest.list_schemas":
		return h.listSchemas(r)
	case "prest.list_tables":
		return h.listTables(r)
	case "prest.describe_table":
		var args mcpDescribeArgs
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
			return nil, fmt.Errorf("invalid describe arguments: %w", err)
		}
		return h.describeTable(r, args)
	case "prest.select_table":
		var args mcpSelectArgs
		if err := json.Unmarshal(payload.Arguments, &args); err != nil {
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
			if err := json.Unmarshal(payload.Arguments, &args); err != nil {
				return nil, fmt.Errorf("invalid select arguments: %w", err)
			}
		}
		return h.selectTable(r, args)
	}
}

func (h *MCPHandler) listDatabases(r *http.Request) (any, error) {
	query, hasCount := h.catalog.DatabaseClause(httptest.NewRequest(http.MethodGet, "/databases", nil))
	query = fmt.Sprint(query, " ", h.catalog.DatabaseWhere(""), " ", h.catalog.DatabaseOrderBy("", hasCount))
	return h.queryRows(r, query)
}

func (h *MCPHandler) listSchemas(r *http.Request) (any, error) {
	query, hasCount := h.catalog.SchemaClause(httptest.NewRequest(http.MethodGet, "/schemas", nil))
	query = fmt.Sprint(query, " ", h.catalog.SchemaOrderBy("", hasCount))
	return h.queryRows(r, query)
}

func (h *MCPHandler) listTables(r *http.Request) (any, error) {
	query := fmt.Sprint(h.catalog.TableClause(), " ", h.catalog.TableWhere(""), " ", h.catalog.TableOrderBy(""))
	return h.queryRows(r, query)
}

func (h *MCPHandler) describeTable(r *http.Request, args mcpDescribeArgs) (any, error) {
	if err := h.validateToolTarget(args.Database, args.Schema, args.Table); err != nil {
		return nil, err
	}

	ctx, cancel := requestContext(r, args.Database)
	defer cancel()

	sc := h.executor.ShowTableCtx(ctx, args.Schema, args.Table)
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("describe table failed: %w", err)
	}

	rows, err := decodeJSONRows(sc.Bytes())
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"database": args.Database,
		"schema":   args.Schema,
		"table":    args.Table,
		"columns":  columnNames(rows),
		"rows":     rows,
	}, nil
}

func (h *MCPHandler) selectTable(r *http.Request, args mcpSelectArgs) (any, error) {
	if args.Limit <= 0 || args.Limit > mcpMaxRows {
		args.Limit = mcpMaxRows
	}
	if args.Offset < 0 {
		args.Offset = 0
	}
	if err := h.validateToolTarget(args.Database, args.Schema, args.Table); err != nil {
		return nil, err
	}

	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d OFFSET %d", quotePathSegment(args.Schema), quotePathSegment(args.Table), args.Limit, args.Offset)
	ctx, cancel := requestContext(r, args.Database)
	defer cancel()

	sc := h.executor.QueryCtx(ctx, query)
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
		Columns:  columnNames(rows),
		Rows:     rows,
		Count:    len(rows),
	}, nil
}

func (h *MCPHandler) tools(r *http.Request) ([]mcpTool, error) {
	tools := []mcpTool{
		{Name: "prest.list_databases", Description: "List accessible databases.", InputSchema: emptyObjectSchema()},
		{Name: "prest.list_schemas", Description: "List schemas from the current database.", InputSchema: emptyObjectSchema()},
		{Name: "prest.list_tables", Description: "List tables from the current database.", InputSchema: emptyObjectSchema()},
		{Name: "prest.describe_table", Description: "Describe a table and return its columns.", InputSchema: mcpDescribeSchema()},
		{Name: "prest.select_table", Description: "Read rows from a table in a read-only way.", InputSchema: mcpSelectSchema()},
	}

	defaultDB := h.defaultDatabase()
	if defaultDB == "" {
		return tools, nil
	}

	rows, err := h.tableRows(r)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		schema := firstString(row, "schema", "schemaname")
		table := firstString(row, "name", "tablename", "table_name")
		if schema == "" || table == "" {
			continue
		}

		desc, err := h.describeToolDescription(r, defaultDB, schema, table)
		if err != nil {
			continue
		}

		name := strings.Join([]string{strings.TrimSuffix(mcpSelectPrefix, "."), defaultDB, schema, table}, ".")
		tools = append(tools, mcpTool{Name: name, Description: desc, InputSchema: mcpLimitOffsetSchema()})
	}

	sort.SliceStable(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

func (h *MCPHandler) describeToolDescription(r *http.Request, database, schema, table string) (string, error) {
	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.ShowTableCtx(ctx, schema, table)
	if err := sc.Err(); err != nil {
		return "", err
	}

	rows, err := decodeJSONRows(sc.Bytes())
	if err != nil {
		return "", err
	}

	columns := columnNames(rows)
	if len(columns) == 0 {
		return fmt.Sprintf("Read rows from %s.%s.%s.", database, schema, table), nil
	}
	return fmt.Sprintf("Read rows from %s.%s.%s. Columns: %s.", database, schema, table, strings.Join(columns, ", ")), nil
}

func (h *MCPHandler) tableRows(r *http.Request) ([]map[string]any, error) {
	query := fmt.Sprint(h.catalog.TableClause(), " ", h.catalog.TableWhere(""), " ", h.catalog.TableOrderBy(""))
	result, err := h.queryRows(r, query)
	if err != nil {
		return nil, err
	}
	rows, ok := result.([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected table discovery result")
	}
	return rows, nil
}

func (h *MCPHandler) queryRows(r *http.Request, query string) (any, error) {
	db := h.defaultDatabase()
	ctx, cancel := requestContext(r, db)
	defer cancel()

	sc := h.executor.QueryCtx(ctx, query)
	if err := sc.Err(); err != nil {
		return nil, err
	}
	rows, err := decodeJSONRows(sc.Bytes())
	if err != nil {
		return nil, err
	}
	return rows, nil
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

func quotePathSegment(segment string) string {
	return `"` + strings.ReplaceAll(segment, `"`, `""`) + `"`
}

func emptyObjectSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
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
		},
		"required": []string{"database", "schema", "table"},
	}
}

func mcpLimitOffsetSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": mcpMaxRows},
			"offset": map[string]any{"type": "integer", "minimum": 0},
		},
	}
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
