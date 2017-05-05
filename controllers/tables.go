package controllers

import (
	"fmt"
	"net/http"

	"encoding/json"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/helpers"
	"github.com/nuveo/prest/statements"
)

// GetTables list all (or filter) tables
func GetTables(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}

	order, err := postgres.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		helpers.ErrorHandler(w, err)
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
		helpers.ErrorHandler(w, err)
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
		helpers.ErrorHandler(w, err)
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
		helpers.ErrorHandler(w, err)
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
		helpers.ErrorHandler(w, err)
		return
	}

	sqlSchemaTables = fmt.Sprint(sqlSchemaTables, " ", page)

	valuesAux := make([]interface{}, 0)
	valuesAux = append(valuesAux, database)
	valuesAux = append(valuesAux, schema)
	valuesAux = append(valuesAux, values...)

	object, err := postgres.Query(sqlSchemaTables, valuesAux...)
	if err != nil {
		helpers.ErrorHandler(w, err)
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
	cols := postgres.ColumnsByRequest(r)
	cols = postgres.FieldsPermissions(table, cols, "read")

	if len(cols) == 0 {
		err := fmt.Errorf("you don't have permission for this action, please check the permitted fields for this table")
		helpers.ErrorHandler(w, err)
		return
	}

	selectStr, err := postgres.SelectFields(cols)
	if err != nil {
		helpers.ErrorHandler(w, err)
		return
	}

	query := fmt.Sprintf("%s %s.%s.%s", selectStr, database, schema, table)

	countQuery, err := postgres.CountByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform CountByRequest: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}
	if countQuery != "" {
		query = fmt.Sprintf("%s %s.%s.%s", countQuery, database, schema, table)
	}

	joinValues, err := postgres.JoinByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform JoinByRequest: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}

	for _, j := range joinValues {
		query = fmt.Sprint(query, j)
	}

	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}

	sqlSelect := query
	if requestWhere != "" {
		sqlSelect = fmt.Sprint(
			query,
			" WHERE ",
			requestWhere)
	}

	order, err := postgres.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}
	if order != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, order)
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}
	sqlSelect = fmt.Sprint(sqlSelect, " ", page)

	runQuery := postgres.Query
	if countQuery != "" {
		runQuery = postgres.QueryCount
	}

	object, err := runQuery(sqlSelect, values...)
	if err != nil {
		helpers.ErrorHandler(w, err)
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

	req := api.Request{}
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		err = fmt.Errorf("could not decode body: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}
	object, err := postgres.Insert(database, schema, table, req)
	if err != nil {
		err = fmt.Errorf("could not perform InsertInTables: %v", err)
		helpers.ErrorHandler(w, err)
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
		helpers.ErrorHandler(w, err)
		return
	}

	object, err := postgres.Delete(database, schema, table, where, values)
	if err != nil {
		err = fmt.Errorf("could not perform DELETE: %v", err)
		helpers.ErrorHandler(w, err)
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

	req := api.Request{}
	defer r.Body.Close()
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		err = fmt.Errorf("could not decode body: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}

	where, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform WhereByRequest: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}

	object, err := postgres.Update(database, schema, table, where, values, req)
	if err != nil {
		err = fmt.Errorf("could not perform UPDATE: %v", err)
		helpers.ErrorHandler(w, err)
		return
	}

	w.Write(object)
}
