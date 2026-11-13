package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/prest/prest/v2/adapters"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/controllers/auth"
	"github.com/prest/prest/v2/internal/logsafe"
	"github.com/prest/prest/v2/middlewares"

	"github.com/structy/log"
)

// CRUDHandler serves table CRUD endpoints.
type CRUDHandler struct {
	builder  adapters.RequestQueryBuilder
	sql      adapters.SQLBuilder
	executor adapters.QueryExecutor
	perms    adapters.PermissionsChecker
	db       adapters.DatabaseRegistry
	cache    ResponseCacher

	allowUnscopedWrites bool
}

func (h *CRUDHandler) ports(r *http.Request) (builder adapters.RequestQueryBuilder, sql adapters.SQLBuilder, executor adapters.QueryExecutor, perms adapters.PermissionsChecker, db adapters.DatabaseRegistry) {
	if a := GetAdapterForRequest(r, nil); a != nil {
		return a, a, a, a, a
	}
	return h.builder, h.sql, h.executor, h.perms, h.db
}

func (h *CRUDHandler) bound(r *http.Request) *CRUDHandler {
	builder, sql, executor, perms, db := h.ports(r)
	cp := *h
	cp.builder, cp.sql, cp.executor, cp.perms, cp.db = builder, sql, executor, perms, db
	return &cp
}

// NewCRUDHandler creates a CRUDHandler.
func NewCRUDHandler(deps Deps) *CRUDHandler {
	return &CRUDHandler{
		builder:  deps.Builder,
		sql:      deps.SQL,
		executor: deps.Executor,
		perms:    deps.Perms,
		db:       deps.DB,
		cache:    deps.Cache,

		allowUnscopedWrites: deps.AllowUnscopedWrites,
	}
}

// Select performs a SELECT on a table.
func (h *CRUDHandler) Select(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]
	queries := r.URL.Query()

	if err := validateDatabase(database, h.db); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateSchema(r, database, schema); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	if !validatePathSegments(database, schema, table) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	userInfo := r.Context().Value(pctx.UserInfoKey)
	var userName string
	if userInfo != nil {
		if user, ok := userInfo.(auth.User); ok {
			userName = user.Username
		}
	}

	cols, err := h.perms.FieldsPermissions(r, database, schema, table, "read", userName)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(cols) == 0 {
		err := errors.New("you don't have permission for this action, please check the permitted fields for this table")
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Filters, _or, _count, _groupby/HAVING and _order must only name columns
	// the caller can read, or they leak values (GHSA-48r9-g6f9-x3gm).
	unreadable, err := h.unreadableColumn(r, database, schema, table, userName)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if unreadable != "" {
		jsonError(w, fmt.Sprintf(columnNotReadableMsg, unreadable), http.StatusForbidden)
		return
	}

	selectStr, err := h.sql.SelectFields(cols)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := h.sql.SelectSQL(selectStr, database, schema, table)

	distinct, err := h.builder.DistinctClause(r)
	if err != nil {
		err = fmt.Errorf("could not perform Distinct: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		query = strings.Replace(query, "SELECT", distinct, 1)
	}

	// CountByRequest re-reads _select; hand it the ACL-filtered list so the
	// count projection drops what the select list drops (GHSA-48r9-g6f9-x3gm).
	countReq := r
	if queries.Get("_select") != "" {
		countReq = countRequest(r, cols)
	}
	countQuery, err := h.builder.CountByRequest(countReq)
	if err != nil {
		err = fmt.Errorf("could not perform CountByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	countFirst := false
	if countQuery != "" {
		query = h.sql.SelectSQL(countQuery, database, schema, table)
		if queries.Get("_count_first") != "" {
			countFirst = true
		}
	}

	joinValues, err := h.builder.JoinByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform JoinByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, j := range joinValues {
		query = fmt.Sprint(query, j)
	}

	requestWhere, values, err := h.builder.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	sqlSelect := query
	if requestWhere != "" {
		sqlSelect = fmt.Sprint(query, " WHERE ", requestWhere)
	}

	groupBySQL, groupValues := adapters.GroupByFromRequest(h.builder, r, len(values)+1)
	if groupBySQL != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, groupBySQL)
	}
	values = append(values, groupValues...)

	timeBucketSQL, err := h.builder.TimeBucketClause(r)
	if err != nil {
		err = fmt.Errorf("could not perform TimeBucketClause: %w", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if timeBucketSQL != "" {
		if groupBySQL != "" {
			bucketExpr := strings.TrimSpace(strings.TrimPrefix(timeBucketSQL, "GROUP BY"))
			sqlSelect = fmt.Sprintf("%s, %s", sqlSelect, bucketExpr)
		} else {
			sqlSelect = fmt.Sprintf("%s %s", sqlSelect, timeBucketSQL)
		}
	}

	order, err := h.builder.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if order != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, order)
	}

	page, err := h.builder.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	sqlSelect = fmt.Sprint(sqlSelect, " ", page)

	ctx, cancel := requestContext(r, database)
	defer cancel()

	runQuery := h.executor.QueryCtx
	if countFirst {
		runQuery = h.executor.QueryCountCtx
	}
	sc := runQuery(ctx, sqlSelect, values...)
	if err = sc.Err(); err != nil {
		log.Errorln(logsafe.Error(err))
		writeStatementError(w, err, schema, table, "", "")
		return
	}

	if r.Method == "GET" && h.cache != nil {
		h.cache.BuntSet(middlewares.CacheKey(r), string(sc.Bytes()))
	}
	//nolint
	w.Write(sc.Bytes())
}

// Insert performs an INSERT on a table.
func (h *CRUDHandler) Insert(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if err := validateDatabase(database, h.db); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateSchema(r, database, schema); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	if !validatePathSegments(database, schema, table) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	if h.rejectUnwritableFields(w, r, database, schema, table, false) {
		return
	}
	readable, all, err := h.readableColumns(r, database, schema, table)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	names, placeholders, values, err := h.builder.ParseInsertRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform InsertInTables: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sql := h.sql.InsertSQL(database, schema, table, names, placeholders)

	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.InsertCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		writeStatementError(w, err, schema, table, "relation does not exist: ", "could not perform InsertInTables: ")
		return
	}
	writeFiltered(w, http.StatusCreated, sc.Bytes(), readable, all)
}

// BatchInsert performs a batch INSERT on a table.
func (h *CRUDHandler) BatchInsert(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if err := validateDatabase(database, h.db); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateSchema(r, database, schema); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	if !validatePathSegments(database, schema, table) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	if h.rejectUnwritableFields(w, r, database, schema, table, true) {
		return
	}
	readable, all, err := h.readableColumns(r, database, schema, table)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	names, placeholders, values, err := h.builder.ParseBatchInsertRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform BatchInsertInTables: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := requestContext(r, database)
	defer cancel()

	var sc adapters.Scanner
	method := r.Header.Get("Prest-Batch-Method")
	if strings.ToLower(method) != "copy" {
		sql := h.sql.InsertSQL(database, schema, table, names, placeholders)
		sc = h.executor.BatchInsertValuesCtx(ctx, sql, values...)
	} else {
		sc = h.executor.BatchInsertCopyCtx(ctx, database, schema, table, strings.Split(names, ","), values...)
	}
	if err = sc.Err(); err != nil {
		writeStatementError(w, err, schema, table, "relation does not exist: ", "could not perform BatchInsertInTables: ")
		return
	}
	writeFiltered(w, http.StatusCreated, sc.Bytes(), readable, all)
}

// Delete performs a DELETE on a table.
func (h *CRUDHandler) Delete(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if err := validateDatabase(database, h.db); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateSchema(r, database, schema); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	if !validatePathSegments(database, schema, table) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	where, values, err := h.builder.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.rejectUnscopedWrite(w, r, where) {
		return
	}

	sql := h.sql.DeleteSQL(database, schema, table)
	if where != "" {
		sql = fmt.Sprint(sql, " WHERE ", where)
	}

	returningSyntax, err := h.builder.ReturningByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform ReturningByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if returningSyntax != "" {
		sql = fmt.Sprint(sql, " RETURNING ", returningSyntax)
	}

	readable, all, err := h.returningColumns(r, database, schema, table, returningSyntax)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.DeleteCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		writeStatementError(w, err, schema, table, "relation does not exist: ", "could not perform DeleteFromTable: ")
		return
	}
	writeFiltered(w, http.StatusOK, sc.Bytes(), readable, all)
}

// Update performs an UPDATE on a table.
func (h *CRUDHandler) Update(w http.ResponseWriter, r *http.Request) {
	h = h.bound(r)
	vars := pathVars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if err := validateDatabase(database, h.db); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateSchema(r, database, schema); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	if !validatePathSegments(database, schema, table) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	if h.rejectUnwritableFields(w, r, database, schema, table, false) {
		return
	}

	setSyntax, values, err := h.builder.SetByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform UPDATE: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	sql := h.sql.UpdateSQL(database, schema, table, setSyntax)

	pid := len(values) + 1

	where, whereValues, err := h.builder.WhereByRequest(r, pid)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.rejectUnscopedWrite(w, r, where) {
		return
	}

	if where != "" {
		sql = fmt.Sprint(sql, " WHERE ", where)
		values = append(values, whereValues...)
	}

	returningSyntax, err := h.builder.ReturningByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform ReturningByRequest: %v", err)
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if returningSyntax != "" {
		sql = fmt.Sprint(sql, " RETURNING ", returningSyntax)
	}

	readable, all, err := h.returningColumns(r, database, schema, table, returningSyntax)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := requestContext(r, database)
	defer cancel()

	sc := h.executor.UpdateCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		writeStatementError(w, err, schema, table, "", "")
		return
	}
	writeFiltered(w, http.StatusOK, sc.Bytes(), readable, all)
}

// rejectUnscopedWrite writes 400 and returns true when a DELETE/PUT/PATCH has
// no WHERE condition, unless access.allow_unscoped_writes is set. Without it an
// empty or filter-less query string would modify every row (GHSA-r867-ghgv-mrp9).
func (h *CRUDHandler) rejectUnscopedWrite(w http.ResponseWriter, r *http.Request, where string) bool {
	if where != "" || h.allowUnscopedWrites {
		return false
	}
	jsonError(w,
		fmt.Sprintf(
			"refusing %s without a WHERE condition: add a filter query parameter",
			r.Method),
		http.StatusBadRequest)
	return true
}

func isRelationNotFound(err error, schema, table string) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, adapters.ErrRelationNotFound) {
		return true
	}
	return strings.Contains(err.Error(), fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table))
}
