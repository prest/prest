package controllers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetTables list all (or filter) tables
func GetTables(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	order, err := postgres.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if order == "" {
		order = statements.TablesOrderBy
	}

	sqlTables := fmt.Sprint(
		statements.TablesSelect,
		statements.TablesWhere)

	if requestWhere != "" {
		sqlTables = fmt.Sprintf("%s AND %s", sqlTables, requestWhere)
	}

	sqlTables = fmt.Sprint(sqlTables, order)

	object, err := postgres.Query(sqlTables, values...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(object)
}

// GetTablesByDatabaseAndSchema list all (or filter) tables based on database and schema
func GetTablesByDatabaseAndSchema(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]

	requestWhere, values, err := postgres.WhereByRequest(r, 3)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemaTables := fmt.Sprint(
		statements.SchemaTablesSelect,
		statements.SchemaTablesWhere)

	if requestWhere != "" {
		sqlSchemaTables = fmt.Sprint(sqlSchemaTables, " AND ", requestWhere)
	}

	order, err := postgres.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if order != "" {
		sqlSchemaTables = fmt.Sprint(sqlSchemaTables, order)
	} else {
		sqlSchemaTables = fmt.Sprint(sqlSchemaTables, statements.SchemaTablesOrderBy)
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemaTables = fmt.Sprint(sqlSchemaTables, " ", page)

	valuesAux := make([]interface{}, 0)
	valuesAux = append(valuesAux, database)
	valuesAux = append(valuesAux, schema)
	valuesAux = append(valuesAux, values...)

	object, err := postgres.Query(sqlSchemaTables, valuesAux...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(object)
}

// SelectFromTables perform select in database
func SelectFromTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	// get selected columns, "*" if empty "_columns"
	cols := postgres.FieldsPermissions(r, table, "read")

	if len(cols) == 0 {
		err := fmt.Errorf("you don't have permission for this action, please check the permitted fields for this table")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	selectStr, err := postgres.SelectFields(cols)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := fmt.Sprintf("%s %s.%s.%s", selectStr, database, schema, table)

	countQuery, err := postgres.CountByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform CountByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if countQuery != "" {
		query = fmt.Sprintf("%s %s.%s.%s", countQuery, database, schema, table)
	}

	joinValues, err := postgres.JoinByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform JoinByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, j := range joinValues {
		query = fmt.Sprint(query, j)
	}

	requestWhere, values, err := postgres.WhereByRequest(r, 1)
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

	groupBySQL := postgres.GroupByClause(r)

	if groupBySQL != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, groupBySQL)
	}

	order, err := postgres.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if order != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, order)
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sqlSelect = fmt.Sprint(sqlSelect, " ", page)

	runQuery := postgres.Query
	if countQuery != "" {
		runQuery = postgres.QueryCount
	}

	object, err := runQuery(sqlSelect, values...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(object)
}

// InsertInTables perform insert in specific table
func InsertInTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	names, placeholders, values, err := postgres.ParseInsertRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform InsertInTables: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sql := fmt.Sprintf(statements.InsertQuery, database, schema, table, names, placeholders)

	object, err := postgres.Insert(sql, values...)
	if err != nil {
		err = fmt.Errorf("could not perform InsertInTables: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(object)
}

// DeleteFromTable perform delete sql
func DeleteFromTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	where, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sql := fmt.Sprintf(statements.DeleteQuery, database, schema, table)
	if where != "" {
		sql = fmt.Sprint(sql, " WHERE ", where)
	}

	object, err := postgres.Delete(sql, values...)
	if err != nil {
		err = fmt.Errorf("could not perform DELETE: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(object)
}

// UpdateTable perform update table
func UpdateTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	where, whereValues, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pid := len(whereValues) + 1 // placeholder id

	setSyntax, values, err := postgres.SetByRequest(r, pid)
	if err != nil {
		err = fmt.Errorf("could not perform UPDATE: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sql := fmt.Sprintf(statements.UpdateQuery, database, schema, table, setSyntax)

	if where != "" {
		sql = fmt.Sprint(
			sql,
			" WHERE ",
			where)
		values = append(whereValues, values...)
	}

	object, err := postgres.Update(sql, values...)
	if err != nil {
		err = fmt.Errorf("could not perform UPDATE: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Write(object)
}
