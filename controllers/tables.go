package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/prest/adapters"
	"github.com/prest/config"
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

	config.PrestConf.Adapter.SetDatabase(database)

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
	valuesAux = append(valuesAux, database)
	valuesAux = append(valuesAux, schema)
	valuesAux = append(valuesAux, values...)
	sc := config.PrestConf.Adapter.Query(sqlSchemaTables, valuesAux...)
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

	config.PrestConf.Adapter.SetDatabase(database)

	// get selected columns, "*" if empty "_columns"
	cols, err := config.PrestConf.Adapter.FieldsPermissions(r, table, "read")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(cols) == 0 {
		err := fmt.Errorf("you don't have permission for this action, please check the permitted fields for this table")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	selectStr, err := config.PrestConf.Adapter.SelectFields(cols)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query := config.PrestConf.Adapter.SelectSQL(selectStr, database, schema, table)

	countQuery, err := config.PrestConf.Adapter.CountByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform CountByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if countQuery != "" {
		query = config.PrestConf.Adapter.SelectSQL(countQuery, database, schema, table)
	}

	joinValues, err := config.PrestConf.Adapter.JoinByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform JoinByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, j := range joinValues {
		query = fmt.Sprint(query, j)
	}

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

	groupBySQL := config.PrestConf.Adapter.GroupByClause(r)

	if groupBySQL != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, groupBySQL)
	}

	order, err := config.PrestConf.Adapter.OrderByRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform OrderByRequest: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if order != "" {
		sqlSelect = fmt.Sprintf("%s %s", sqlSelect, order)
	}

	page, err := config.PrestConf.Adapter.PaginateIfPossible(r)
	if err != nil {
		err = fmt.Errorf("could not perform PaginateIfPossible: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sqlSelect = fmt.Sprint(sqlSelect, " ", page)

	runQuery := config.PrestConf.Adapter.Query
	if countQuery != "" {
		runQuery = config.PrestConf.Adapter.QueryCount
	}

	sc := runQuery(sqlSelect, values...)
	if err = sc.Err(); err != nil {
		errorMessage := sc.Err().Error()
		if errorMessage == fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table) {
			fmt.Println(errorMessage)
			http.Error(w, errorMessage, http.StatusNotFound)
			return
		}
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}

// InsertInTables perform insert in specific table
func InsertInTables(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	schema := vars["schema"]
	table := vars["table"]

	config.PrestConf.Adapter.SetDatabase(database)

	names, placeholders, values, err := config.PrestConf.Adapter.ParseInsertRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform InsertInTables: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sql := config.PrestConf.Adapter.InsertSQL(database, schema, table, names, placeholders)

	sc := config.PrestConf.Adapter.Insert(sql, values...)
	if err = sc.Err(); err != nil {
		errorMessage := sc.Err().Error()
		if errorMessage == fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table) {
			fmt.Println(errorMessage)
			http.Error(w, errorMessage, http.StatusNotFound)
			return
		}
		http.Error(w, errorMessage, http.StatusBadRequest)
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

	config.PrestConf.Adapter.SetDatabase(database)

	names, placeholders, values, err := config.PrestConf.Adapter.ParseBatchInsertRequest(r)
	if err != nil {
		err = fmt.Errorf("could not perform BatchInsertInTables: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var sc adapters.Scanner
	method := r.Header.Get("Prest-Batch-Method")
	if strings.ToLower(method) != "copy" {
		sql := config.PrestConf.Adapter.InsertSQL(database, schema, table, names, placeholders)
		sc = config.PrestConf.Adapter.BatchInsertValues(sql, values...)
	} else {
		sc = config.PrestConf.Adapter.BatchInsertCopy(database, schema, table, strings.Split(names, ","), values...)
	}
	if err = sc.Err(); err != nil {
		errorMessage := sc.Err().Error()
		if errorMessage == fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table) {
			fmt.Println(errorMessage)
			http.Error(w, errorMessage, http.StatusNotFound)
			return
		}
		http.Error(w, errorMessage, http.StatusBadRequest)
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

	config.PrestConf.Adapter.SetDatabase(database)

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

	sc := config.PrestConf.Adapter.Delete(sql, values...)
	if err = sc.Err(); err != nil {
		errorMessage := sc.Err().Error()
		if errorMessage == fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table) {
			fmt.Println(errorMessage)
			http.Error(w, errorMessage, http.StatusNotFound)
			return
		}
		http.Error(w, errorMessage, http.StatusBadRequest)
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

	config.PrestConf.Adapter.SetDatabase(database)

	setSyntax, values, err := config.PrestConf.Adapter.SetByRequest(r, 1)
	if err != nil {
		err = fmt.Errorf("could not perform UPDATE: %v", err)
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

	sc := config.PrestConf.Adapter.Update(sql, values...)
	if err = sc.Err(); err != nil {
		errorMessage := sc.Err().Error()
		if errorMessage == fmt.Sprintf(`pq: relation "%s.%s" does not exist`, schema, table) {
			fmt.Println(errorMessage)
			http.Error(w, errorMessage, http.StatusNotFound)
			return
		}
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}
	w.Write(sc.Bytes())
}
