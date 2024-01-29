package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	slog "github.com/structy/log"

	"github.com/prest/prest/adapters/scanner"
	pctx "github.com/prest/prest/context"
)

var (
	ErrNoPermissions        = errors.New("you don't have permission for this action, please check the permitted fields for this table")
	ErrDatabaseNotAllowed   = errors.New("database not allowed")
	ErrCouldNotPerformQuery = errors.New("could not perform query, check logs for more details")
	ErrRelationDoesntExist  = errors.New("relation does not exist")
)

// GetTables list all (or filter) tables
func (c *Config) GetTables(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := c.adapter.WhereByRequest(r, 1)
	if err != nil {
		slog.Errorln("could not perform WhereByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = c.adapter.TableWhere(requestWhere)

	order, err := c.adapter.OrderByRequest(r)
	if err != nil {
		slog.Errorln("could not perform OrderByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = c.adapter.TableOrderBy(order)

	sqlTables := c.adapter.TableClause()

	distinct, err := c.adapter.DistinctClause(r)
	if err != nil {
		slog.Errorln("could not perform DistinctClause", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlTables = strings.Replace(sqlTables, "SELECT", distinct, 1)
	}

	sqlTables = fmt.Sprint(sqlTables, requestWhere, order)

	ctx, cancel := pctx.WithTimeout(r.Context())
	defer cancel()

	sc := c.adapter.QueryCtx(ctx, sqlTables, values...)
	if err = sc.Err(); err != nil {
		slog.Errorln("could not execute query", err)
		JSONError(w, ErrCouldNotPerformQuery.Error(), http.StatusBadRequest)
		return
	}

	slog.Debugln("[GetTables] query executed successfully")
	JSONWrite(w, string(sc.Bytes()), http.StatusOK)
}

// GetTablesByDatabaseAndSchema list all (or filter) tables based on database and schema
func (c *Config) GetTablesByDatabaseAndSchema(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]

	if c.differentDbQuery(database) {
		slog.Errorln("database not allowed", database)
		JSONError(w, ErrDatabaseNotAllowed.Error(), http.StatusBadRequest)
		return
	}

	requestWhere, values, err := c.adapter.WhereByRequest(r, 3)
	if err != nil {
		slog.Errorln("could not perform WhereByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = c.adapter.SchemaTablesWhere(requestWhere)

	sqlSchemaTables := c.adapter.SchemaTablesClause()

	order, err := c.adapter.OrderByRequest(r)
	if err != nil {
		slog.Errorln("could not perform OrderByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = c.adapter.SchemaTablesOrderBy(order)

	page, err := c.adapter.PaginateIfPossible(r)
	if err != nil {
		slog.Errorln("could not perform PaginateIfPossible", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemaTables = fmt.Sprint(sqlSchemaTables, requestWhere, order, " ", page)

	valuesAux := make([]interface{}, 0)
	valuesAux = append(valuesAux, database, schema)
	valuesAux = append(valuesAux, values...)

	ctx, cancel := pctx.WithTimeout(
		context.WithValue(r.Context(), pctx.DBNameKey, database))
	defer cancel()

	// send ctx to query the proper DB
	sc := c.adapter.QueryCtx(ctx, sqlSchemaTables, valuesAux...)
	if err = sc.Err(); err != nil {
		slog.Errorln("could not execute query", err)
		JSONError(w, ErrCouldNotPerformQuery.Error(), http.StatusBadRequest)
		return
	}

	slog.Debugln("[GetTablesByDatabaseAndSchema] query executed successfully")
	JSONWrite(w, string(sc.Bytes()), http.StatusOK)
}

// SelectFromTables perform select in database
func (c *Config) SelectFromTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]
	queries := r.URL.Query()

	if c.differentDbQuery(database) {
		slog.Errorln("database not allowed", database)
		JSONError(w, ErrDatabaseNotAllowed.Error(), http.StatusBadRequest)
		return
	}

	// get selected columns, "*" if empty "_columns"
	cols, err := c.adapter.FieldsPermissions(r, table, "read")
	if err != nil {
		slog.Errorln("could not perform FieldsPermissions", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(cols) == 0 {
		err := ErrNoPermissions
		slog.Errorln("could not perform FieldsPermissions", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	selectStr, err := c.adapter.SelectFields(cols)
	if err != nil {
		slog.Errorln("could not perform SelectFields", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := c.adapter.SelectSQL(selectStr, database, schema, table)

	// sql query formatting if there is a distinct rule
	distinct, err := c.adapter.DistinctClause(r)
	if err != nil {
		err = fmt.Errorf("could not perform Distinct: %v", err)
		slog.Errorln("distinct error", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		query = strings.Replace(query, "SELECT", distinct, 1)
	}

	// sql query formatting if there is a count rule
	countQuery, err := c.adapter.CountByRequest(r)
	if err != nil {
		slog.Errorln("could not perform CountByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	// _count_first: query string
	countFirst := false
	if countQuery != "" {
		query = c.adapter.SelectSQL(countQuery, database, schema, table)
		// count returns a list, passing this parameter will return the first
		// record as a non-list object
		if queries.Get("_count_first") != "" {
			countFirst = true
		}
	}

	// sql query formatting if there is a join (inner, left, ...) rule
	joinValues, err := c.adapter.JoinByRequest(r)
	if err != nil {
		slog.Errorln("could not perform JoinByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, j := range joinValues {
		query = fmt.Sprint(query, j)
	}

	// sql query formatting if there is a where rule
	requestWhere, values, err := c.adapter.WhereByRequest(r, 1)
	if err != nil {
		slog.Errorln("could not perform WhereByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	sqlSelect := query
	if requestWhere != "" {
		sqlSelect = fmt.Sprint(
			query,
			" WHERE ",
			requestWhere)
	}

	// sql query formatting if there is a groupby rule
	groupBySQL := c.adapter.GroupByClause(r)
	if groupBySQL != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, groupBySQL)
	}

	// sql query formatting if there is a orderby rule
	order, err := c.adapter.OrderByRequest(r)
	if err != nil {
		slog.Errorln("could not perform OrderByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if order != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, order)
	}

	// sql query formatting if there is a paganate rule
	page, err := c.adapter.PaginateIfPossible(r)
	if err != nil {
		slog.Errorln("could not perform PaginateIfPossible", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	sqlSelect = fmt.Sprint(sqlSelect, " ", page)

	ctx, cancel := pctx.WithTimeout(
		context.WithValue(r.Context(), pctx.DBNameKey, database))
	defer cancel()

	runQuery := c.adapter.QueryCtx
	// QueryCount returns the first record of the postgresql return as a non-list object
	if countFirst {
		runQuery = c.adapter.QueryCountCtx
	}
	sc := runQuery(ctx, sqlSelect, values...)
	if err = sc.Err(); err != nil {
		errMsg := err.Error()
		slog.Errorln("could not execute query", err)

		if strings.Contains(errMsg,
			fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			JSONError(w, ErrRelationDoesntExist.Error(), http.StatusNotFound)
			return
		}

		JSONError(w, ErrCouldNotPerformQuery.Error(), http.StatusNotFound)
		return
	}

	// Cache arrow if enabled
	c.cache.Set(r.URL.String(), string(sc.Bytes()))

	JSONWrite(w, string(sc.Bytes()), http.StatusOK)
}

// InsertInTables perform insert in specific table
func (c *Config) InsertInTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if c.differentDbQuery(database) {
		slog.Errorln("database not allowed", database)
		JSONError(w, ErrDatabaseNotAllowed.Error(), http.StatusBadRequest)
		return
	}

	names, placeholders, values, err := c.adapter.ParseInsertRequest(r)
	if err != nil {
		slog.Errorln("could not perform ParseInsertRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sql := c.adapter.InsertSQL(database, schema, table, names, placeholders)

	ctx, cancel := pctx.WithTimeout(
		context.WithValue(r.Context(), pctx.DBNameKey, database))
	defer cancel()

	sc := c.adapter.InsertCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		errMsg := err.Error()
		slog.Errorln("could not execute query", err)

		if strings.Contains(errMsg,
			fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			JSONError(w, ErrRelationDoesntExist.Error(), http.StatusNotFound)
			return
		}

		JSONError(w, ErrCouldNotPerformQuery.Error(), http.StatusNotFound)
		return
	}

	slog.Debugln("[InsertInTables] query executed successfully")
	JSONWrite(w, string(sc.Bytes()), http.StatusCreated)
}

// BatchInsertInTables perform insert in specific table from a batch request
func (c *Config) BatchInsertInTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if c.differentDbQuery(database) {
		slog.Errorln("database not allowed", database)
		JSONError(w, ErrDatabaseNotAllowed.Error(), http.StatusBadRequest)
		return
	}

	names, placeholders, values, err := c.adapter.ParseBatchInsertRequest(r)
	if err != nil {
		slog.Errorln("could not perform ParseBatchInsertRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := pctx.WithTimeout(
		context.WithValue(r.Context(), pctx.DBNameKey, database))
	defer cancel()

	var sc scanner.Scanner
	method := r.Header.Get("Prest-Batch-Method")
	if strings.ToLower(method) != "copy" {
		sql := c.adapter.InsertSQL(database, schema, table, names, placeholders)
		sc = c.adapter.BatchInsertValuesCtx(ctx, sql, values...)
	} else {
		sc = c.adapter.BatchInsertCopyCtx(ctx, database, schema, table,
			strings.Split(names, ","), values...)
	}

	if err = sc.Err(); err != nil {
		errMsg := err.Error()
		slog.Errorln("could not execute query", err)

		if strings.Contains(errMsg,
			fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			JSONError(w, ErrRelationDoesntExist.Error(), http.StatusNotFound)
			return
		}

		JSONError(w, ErrCouldNotPerformQuery.Error(), http.StatusNotFound)
		return
	}

	slog.Debugln("[BatchInsertInTables] query executed successfully")
	JSONWrite(w, string(sc.Bytes()), http.StatusCreated)
}

// DeleteFromTable perform delete sql
func (c *Config) DeleteFromTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if c.differentDbQuery(database) {
		slog.Errorln("database not allowed", database)
		JSONError(w, ErrDatabaseNotAllowed.Error(), http.StatusBadRequest)
		return
	}

	where, values, err := c.adapter.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	sql := c.adapter.DeleteSQL(database, schema, table)
	if where != "" {
		sql = fmt.Sprint(sql, " WHERE ", where)
	}

	returningSyntax, err := c.adapter.ReturningByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform ReturningByRequest: %v", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if returningSyntax != "" {
		sql = fmt.Sprint(sql, " RETURNING ", returningSyntax)
	}

	ctx, cancel := pctx.WithTimeout(
		context.WithValue(r.Context(), pctx.DBNameKey, database))
	defer cancel()

	sc := c.adapter.DeleteCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		errMsg := err.Error()
		slog.Errorln("could not execute query", err)

		if strings.Contains(errMsg,
			fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			JSONError(w, ErrRelationDoesntExist.Error(), http.StatusNotFound)
			return
		}

		JSONError(w, ErrCouldNotPerformQuery.Error(), http.StatusNotFound)
		return
	}

	slog.Debugln("[DeleteFromTable] query executed successfully")
	JSONWrite(w, string(sc.Bytes()), http.StatusOK)
}

// UpdateTable perform update table
func (c *Config) UpdateTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if c.differentDbQuery(database) {
		slog.Errorln("database not allowed", database)
		JSONError(w, ErrDatabaseNotAllowed.Error(), http.StatusBadRequest)
		return
	}

	setSyntax, values, err := c.adapter.SetByRequest(r, 1)
	if err != nil {
		slog.Errorln("could not perform SetByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	sql := c.adapter.UpdateSQL(database, schema, table, setSyntax)

	pid := len(values) + 1 // placeholder id

	where, whereValues, err := c.adapter.WhereByRequest(r, pid)
	if err != nil {
		slog.Errorln("could not perform WhereByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if where != "" {
		sql = fmt.Sprint(sql, " WHERE ", where)
		values = append(values, whereValues...)
	}

	returningSyntax, err := c.adapter.ReturningByRequest(r)
	if err != nil {
		slog.Errorln("could not perform ReturningByRequest", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if returningSyntax != "" {
		sql = fmt.Sprint(sql, " RETURNING ", returningSyntax)
	}

	ctx, cancel := pctx.WithTimeout(
		context.WithValue(r.Context(), pctx.DBNameKey, database))
	defer cancel()

	sc := c.adapter.UpdateCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		errMsg := err.Error()
		slog.Errorln("could not execute query", err)

		if strings.Contains(errMsg,
			fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			JSONError(w, ErrRelationDoesntExist.Error(), http.StatusNotFound)
			return
		}

		JSONError(w, ErrCouldNotPerformQuery.Error(), http.StatusNotFound)
		return
	}

	slog.Debugln("[UpdateTable] query executed successfully")
	JSONWrite(w, string(sc.Bytes()), http.StatusOK)
}

// ShowTable show information from table
func (c *Config) ShowTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if c.differentDbQuery(database) {
		slog.Errorln("database not allowed", database)
		JSONError(w, ErrDatabaseNotAllowed.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := pctx.WithTimeout(
		context.WithValue(r.Context(), pctx.DBNameKey, database))
	defer cancel()

	sc := c.adapter.ShowTableCtx(ctx, schema, table)
	if err := sc.Err(); err != nil {
		slog.Errorln("could not execute query", err)
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	slog.Debugln("[ShowTable] query executed successfully")
	JSONWrite(w, string(sc.Bytes()), http.StatusOK)
}

// differentDbQuery checks if the query is for the same database
//
// by default the server is a single DB configuration
//
// queries for the same database are allowed and should return false
// queries for different databases are not allowed in this config and should return true
// thus returning an error
//
// should avoid the error if singleDB is disabled
// then querying multiple databases is allowed
func (c *Config) differentDbQuery(database string) bool {
	return c.server.SingleDB &&
		(c.adapter.GetCurrentConnDatabase() != database)
}
