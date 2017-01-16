package controllers

import (
	"fmt"
	"log"
	"net/http"

	"encoding/json"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/statements"
)

// GetTables list all (or filter) tables
func GetTables(w http.ResponseWriter, r *http.Request) {
	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	order, _ := postgres.OrderByRequest(r)
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
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}

// GetTablesByDatabaseAndSchema list all (or filter) tables based on database and schema
func GetTablesByDatabaseAndSchema(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database, ok := vars["database"]
	if !ok {
		log.Println("Unable to parse database in URI")
		http.Error(w, "Unable to parse database in URI", http.StatusInternalServerError)
		return
	}
	schema, ok := vars["schema"]
	if !ok {
		log.Println("Unable to parse schema in URI")
		http.Error(w, "Unable to parse schema in URI", http.StatusInternalServerError)
		return
	}
	requestWhere, values, err := postgres.WhereByRequest(r, 3)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlSchemaTables := fmt.Sprint(
		statements.SchemaTablesSelect,
		statements.SchemaTablesWhere)

	if requestWhere != "" {
		sqlSchemaTables = fmt.Sprint(sqlSchemaTables, " AND ", requestWhere)
	}

	order, _ := postgres.OrderByRequest(r)
	if order != "" {
		sqlSchemaTables = fmt.Sprint(sqlSchemaTables, order)
	} else {
		sqlSchemaTables = fmt.Sprint(sqlSchemaTables, statements.SchemaTablesOrderBy)
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, "Paging error", http.StatusBadRequest)
		return
	}

	sqlSchemaTables = fmt.Sprint(sqlSchemaTables, " ", page)

	valuesAux := make([]interface{}, 0)
	valuesAux = append(valuesAux, database)
	valuesAux = append(valuesAux, schema)
	valuesAux = append(valuesAux, values...)

	object, err := postgres.Query(sqlSchemaTables, valuesAux...)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}

// SelectFromTables perform select in database
func SelectFromTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database, ok := vars["database"]
	if !ok {
		log.Println("Unable to parse database in URI")
		http.Error(w, "Unable to parse database in URI", http.StatusInternalServerError)
		return
	}
	schema, ok := vars["schema"]
	if !ok {
		log.Println("Unable to parse schema in URI")
		http.Error(w, "Unable to parse schema in URI", http.StatusInternalServerError)
		return
	}
	table, ok := vars["table"]
	if !ok {
		log.Println("Unable to parse table in URI")
		http.Error(w, "Unable to parse table in URI", http.StatusInternalServerError)
		return
	}

	permission := postgres.TablePermissions(table, "read")
	if !permission {
		log.Println("You don't have permission for this action.")
		http.Error(w, "Unable to parse table in URI", http.StatusMethodNotAllowed)
		return
	}

	// get selected columns, "*" if empty "_columns"
	cols := postgres.ColumnsByRequest(r)
	cols = postgres.FieldsPermissions(table, cols, "read")

	if len(cols) == 0 {
		log.Println("You don't have permission for this action. Please check the permitted fields for this table.")
		http.Error(w, "You don't have permission for this action. Please check the permitted fields for this table.", http.StatusUnauthorized)
		return
	}

	selectStr, _ := postgres.SelectFields(cols)
	query := fmt.Sprintf("%s %s.%s.%s", selectStr, database, schema, table)

	countQuery := postgres.CountByRequest(r)
	if countQuery != "" {
		query = fmt.Sprintf("%s %s.%s.%s", countQuery, database, schema, table)
	}

	joinValues, err := postgres.JoinByRequest(r)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, j := range joinValues {
		query = fmt.Sprint(query, j)
	}

	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		log.Println(err)
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

	order, _ := postgres.OrderByRequest(r)
	if len(order) > 0 {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, order)
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, "Paging error", http.StatusBadRequest)
		return
	}
	sqlSelect = fmt.Sprint(sqlSelect, " ", page)

	runQuery := postgres.Query
	if countQuery != "" {
		runQuery = postgres.QueryCount
	}

	object, err := runQuery(sqlSelect, values...)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}

// InsertInTables perform insert in specific table
func InsertInTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database, ok := vars["database"]
	if !ok {
		log.Println("Unable to parse database in URI")
		http.Error(w, "Unable to parse database in URI", http.StatusInternalServerError)
		return
	}
	schema, ok := vars["schema"]
	if !ok {
		log.Println("Unable to parse schema in URI")
		http.Error(w, "Unable to parse schema in URI", http.StatusInternalServerError)
		return
	}
	table, ok := vars["table"]
	if !ok {
		log.Println("Unable to parse table in URI")
		http.Error(w, "Unable to parse table in URI", http.StatusInternalServerError)
		return
	}
	req := api.Request{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Println("InsertInTables:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	object, err := postgres.Insert(database, schema, table, req)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}

// DeleteFromTable perform delete sql
func DeleteFromTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database, ok := vars["database"]
	if !ok {
		log.Println("Unable to parse database in URI")
		http.Error(w, "Unable to parse database in URI", http.StatusInternalServerError)
		return
	}
	schema, ok := vars["schema"]
	if !ok {
		log.Println("Unable to parse schema in URI")
		http.Error(w, "Unable to parse schema in URI", http.StatusInternalServerError)
		return
	}
	table, ok := vars["table"]
	if !ok {
		log.Println("Unable to parse table in URI")
		http.Error(w, "Unable to parse table in URI", http.StatusInternalServerError)
		return
	}

	where, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	object, err := postgres.Delete(database, schema, table, where, values)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}

// UpdateTable perform update table
func UpdateTable(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database, ok := vars["database"]
	if !ok {
		log.Println("Unable to parse database in URI")
		http.Error(w, "Unable to parse database in URI", http.StatusInternalServerError)
		return
	}
	schema, ok := vars["schema"]
	if !ok {
		log.Println("Unable to parse schema in URI")
		http.Error(w, "Unable to parse schema in URI", http.StatusInternalServerError)
		return
	}
	table, ok := vars["table"]
	if !ok {
		log.Println("Unable to parse table in URI")
		http.Error(w, "Unable to parse table in URI", http.StatusInternalServerError)
		return
	}

	req := api.Request{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	where, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	object, err := postgres.Update(database, schema, table, where, values, req)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}

// SelectFromViews
func SelectFromViews(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database, ok := vars["database"]
	if !ok {
		log.Println("Unable to parse database in URI")
		http.Error(w, "Unable to parse database in URI", http.StatusInternalServerError)
		return
	}
	schema, ok := vars["schema"]
	if !ok {
		log.Println("Unable to parse schema in URI")
		http.Error(w, "Unable to parse schema in URI", http.StatusInternalServerError)
		return
	}
	view, ok := vars["view"]
	if !ok {
		log.Println("Unable to parse view in URI")
		http.Error(w, "Unable to parse view in URI", http.StatusInternalServerError)
		return
	}

	// get selected columns, "*" if empty "_columns"
	cols := postgres.ColumnsByRequest(r)

	selectStr, _ := postgres.SelectFields(cols)
	query := fmt.Sprintf("%s %s.%s.%s", selectStr, database, schema, view)

	countQuery := postgres.CountByRequest(r)
	if countQuery != "" {
		query = fmt.Sprintf("%s %s.%s.%s", countQuery, database, schema, view)
	}

	joinValues, err := postgres.JoinByRequest(r)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, j := range joinValues {
		query = fmt.Sprint(query, j)
	}

	requestWhere, values, err := postgres.WhereByRequest(r, 1)
	if err != nil {
		log.Println(err)
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

	order, _ := postgres.OrderByRequest(r)
	if len(order) > 0 {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, order)
	}

	page, err := postgres.PaginateIfPossible(r)
	if err != nil {
		http.Error(w, "Paging error", http.StatusBadRequest)
		return
	}
	sqlSelect = fmt.Sprint(sqlSelect, " ", page)

	runQuery := postgres.Query
	if countQuery != "" {
		runQuery = postgres.QueryCount
	}

	object, err := runQuery(sqlSelect, values...)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}
