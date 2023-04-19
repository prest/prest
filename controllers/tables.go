package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prest/prest/adapters"
	"github.com/prest/prest/cache"
	"github.com/prest/prest/config"
	pctx "github.com/prest/prest/context"
	"github.com/structy/log"
)

// GetTables list all (or filter) tables
func GetTables(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := config.PrestConf.Adapter.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = config.PrestConf.Adapter.TableWhere(requestWhere)

	order, err := config.PrestConf.Adapter.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = config.PrestConf.Adapter.TableOrderBy(order)

	sqlTables := config.PrestConf.Adapter.TableClause()

	distinct, err := config.PrestConf.Adapter.DistinctClause(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		sqlTables = strings.Replace(sqlTables, "SELECT", distinct, 1)
	}

	sqlTables = fmt.Sprint(sqlTables, requestWhere, order)

	sc := config.PrestConf.Adapter.Query(sqlTables, values...)
	if sc.Err() != nil {
		http.Error(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}

// GetTablesByDatabaseAndSchema list all (or filter) tables based on database and schema
func GetTablesByDatabaseAndSchema(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]

	if config.PrestConf.SingleDB && (config.PrestConf.Adapter.GetDatabase() != database) {
		err := fmt.Errorf("database not registered: %v", database)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	requestWhere, values, err := config.PrestConf.Adapter.WhereByRequest(r, 3)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	requestWhere = config.PrestConf.Adapter.SchemaTablesWhere(requestWhere)

	sqlSchemaTables := config.PrestConf.Adapter.SchemaTablesClause()

	order, err := config.PrestConf.Adapter.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order = config.PrestConf.Adapter.SchemaTablesOrderBy(order)

	page, err := config.PrestConf.Adapter.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemaTables = fmt.Sprint(sqlSchemaTables, requestWhere, order, " ", page)

	valuesAux := make([]interface{}, 0)
	valuesAux = append(valuesAux, database, schema)
	valuesAux = append(valuesAux, values...)

	// set db name on ctx
	ctx := context.WithValue(r.Context(), pctx.DBNameKey, database)

	timeout, _ := ctx.Value(pctx.HTTPTimeoutKey).(int)
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(timeout))
	defer cancel()

	// send ctx to query the proper DB
	sc := config.PrestConf.Adapter.QueryCtx(ctx, sqlSchemaTables, valuesAux...)
	if sc.Err() != nil {
		http.Error(w, sc.Err().Error(), http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}

// SelectFromTables perform select in database
func SelectFromTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]
	queries := r.URL.Query()

	if config.PrestConf.SingleDB && (config.PrestConf.Adapter.GetDatabase() != database) {
		err := fmt.Errorf("database not registered: %v", database)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// get selected columns, "*" if empty "_columns"
	cols, err := config.PrestConf.Adapter.FieldsPermissions(r, table, "read")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(cols) == 0 {
		err := errors.New("you don't have permission for this action, please check the permitted fields for this table")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	selectStr, err := config.PrestConf.Adapter.SelectFields(cols)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := config.PrestConf.Adapter.SelectSQL(selectStr, database, schema, table)

	// sql query formatting if there is a distinct rule
	distinct, err := config.PrestConf.Adapter.DistinctClause(r)
	if err != nil {
		err = fmt.Errorf("could not perform Distinct: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if distinct != "" {
		query = strings.Replace(query, "SELECT", distinct, 1)
	}

	// sql query formatting if there is a count rule
	countQuery, err := config.PrestConf.Adapter.CountByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform CountByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// _count_first: query string
	countFirst := false
	if countQuery != "" {
		query = config.PrestConf.Adapter.SelectSQL(countQuery, database, schema, table)
		// count returns a list, passing this parameter will return the first
		// record as a non-list object
		if queries.Get("_count_first") != "" {
			countFirst = true
		}
	}

	// sql query formatting if there is a join (inner, left, ...) rule
	joinValues, err := config.PrestConf.Adapter.JoinByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform JoinByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, j := range joinValues {
		query = fmt.Sprint(query, j)
	}

	// sql query formatting if there is a where rule
	requestWhere, values, err := config.PrestConf.Adapter.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	groupBySQL := config.PrestConf.Adapter.GroupByClause(r)
	if groupBySQL != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, groupBySQL)
	}

	// sql query formatting if there is a orderby rule
	order, err := config.PrestConf.Adapter.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if order != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, order)
	}

	// sql query formatting if there is a paganate rule
	page, err := config.PrestConf.Adapter.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sqlSelect = fmt.Sprint(sqlSelect, " ", page)

	ctx := context.WithValue(r.Context(), pctx.DBNameKey, database)

	timeout, _ := ctx.Value(pctx.HTTPTimeoutKey).(int)
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(timeout))
	defer cancel()

	runQuery := config.PrestConf.Adapter.QueryCtx
	// QueryCount returns the first record of the postgresql return as a non-list object
	if countFirst {
		runQuery = config.PrestConf.Adapter.QueryCountCtx
	}
	sc := runQuery(ctx, sqlSelect, values...)
	if err = sc.Err(); err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Cache arrow if enabled
	cache.BuntSet(r.URL.String(), string(sc.Bytes()))
	w.Write(sc.Bytes())
}

// InsertInTables perform insert in specific table
func InsertInTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if config.PrestConf.SingleDB && (config.PrestConf.Adapter.GetDatabase() != database) {
		err := fmt.Errorf("database not registered: %v", database)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	names, placeholders, values, err := config.PrestConf.Adapter.ParseInsertRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform InsertInTables: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(names) == 0 {
		err := errors.New("you don't have permission for this action, please check the permitted fields for this table")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sql := config.PrestConf.Adapter.InsertSQL(database, schema, table, names, placeholders)

	// set db name on ctx
	ctx := context.WithValue(r.Context(), pctx.DBNameKey, database)

	timeout, _ := ctx.Value(pctx.HTTPTimeoutKey).(int)
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(timeout))
	defer cancel()

	sc := config.PrestConf.Adapter.InsertCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write(sc.Bytes())
}

// BatchInsertInTables perform insert in specific table from a batch request
func BatchInsertInTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if config.PrestConf.SingleDB && (config.PrestConf.Adapter.GetDatabase() != database) {
		err := fmt.Errorf("database not registered: %v", database)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	names, placeholders, values, err := config.PrestConf.Adapter.ParseBatchInsertRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform BatchInsertInTables: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// set db name on ctx
	ctx := context.WithValue(r.Context(), pctx.DBNameKey, database)

	timeout, _ := ctx.Value(pctx.HTTPTimeoutKey).(int)
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(timeout))
	defer cancel()

	var sc adapters.Scanner
	method := r.Header.Get("Prest-Batch-Method")
	if strings.ToLower(method) != "copy" {
		sql := config.PrestConf.Adapter.InsertSQL(database, schema, table, names, placeholders)
		sc = config.PrestConf.Adapter.BatchInsertValuesCtx(ctx, sql, values...)
	} else {
		sc = config.PrestConf.Adapter.BatchInsertCopyCtx(ctx, database, schema, table, strings.Split(names, ","), values...)
	}
	if err = sc.Err(); err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			log.Println(sc.Err().Error())
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write(sc.Bytes())
}

// DeleteFromTable perform delete sql
func DeleteFromTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if config.PrestConf.SingleDB && (config.PrestConf.Adapter.GetDatabase() != database) {
		err := fmt.Errorf("database not registered: %v", database)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	where, values, err := config.PrestConf.Adapter.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sql := config.PrestConf.Adapter.DeleteSQL(database, schema, table)
	if where != "" {
		sql = fmt.Sprint(sql, " WHERE ", where)
	}

	returningSyntax, err := config.PrestConf.Adapter.ReturningByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform ReturningByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if returningSyntax != "" {
		sql = fmt.Sprint(
			sql,
			" RETURNING ",
			returningSyntax)
	}

	ctx := context.WithValue(r.Context(), pctx.DBNameKey, database)

	timeout, _ := ctx.Value(pctx.HTTPTimeoutKey).(int)
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(timeout))
	defer cancel()

	sc := config.PrestConf.Adapter.DeleteCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			log.Println(sc.Err().Error())
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}

// UpdateTable perform update table
func UpdateTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if config.PrestConf.SingleDB && (config.PrestConf.Adapter.GetDatabase() != database) {
		err := fmt.Errorf("database not registered: %v", database)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	setSyntax, values, err := config.PrestConf.Adapter.SetByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform UPDATE: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(setSyntax) == 0 {
		err := errors.New("you don't have permission for this action, please check the permitted fields for this table")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sql := config.PrestConf.Adapter.UpdateSQL(database, schema, table, setSyntax)

	pid := len(values) + 1 // placeholder id

	where, whereValues, err := config.PrestConf.Adapter.WhereByRequest(r, pid)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if where != "" {
		sql = fmt.Sprint(
			sql,
			" WHERE ",
			where)
		values = append(values, whereValues...)
	}

	returningSyntax, err := config.PrestConf.Adapter.ReturningByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform ReturningByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if returningSyntax != "" {
		sql = fmt.Sprint(
			sql,
			" RETURNING ",
			returningSyntax)
	}
	ctx := context.WithValue(r.Context(), pctx.DBNameKey, database)

	timeout, _ := ctx.Value(pctx.HTTPTimeoutKey).(int)
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(timeout))
	defer cancel()

	sc := config.PrestConf.Adapter.UpdateCtx(ctx, sql, values...)
	if err = sc.Err(); err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table)) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}

// ShowTable show information from table
func ShowTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	if config.PrestConf.SingleDB && (config.PrestConf.Adapter.GetDatabase() != database) {
		err := fmt.Errorf("database not registered: %v", database)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// set db name on ctx
	ctx := context.WithValue(r.Context(), pctx.DBNameKey, database)

	timeout, _ := ctx.Value(pctx.HTTPTimeoutKey).(int)
	ctx, cancel := context.WithTimeout(ctx, time.Second*time.Duration(timeout))
	defer cancel()

	sc := config.PrestConf.Adapter.ShowTableCtx(ctx, schema, table)
	if sc.Err() != nil {
		errorMessage := fmt.Sprintf("error to execute query, schema error %s", sc.Err())
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}
