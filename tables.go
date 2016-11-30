package controllers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetTables list all (or filter) tables
func GetTables(w http.ResponseWriter, r *http.Request) {
	requestWhere := postgres.WhereByRequest(r)
	sqlTables := statements.Tables
	if requestWhere != "" {
		sqlTables = fmt.Sprint(
			statements.TablesSelect,
			statements.TablesWhere,
			" AND ",
			requestWhere,
			statements.TablesOrderBy)
	}

	object, err := postgres.Query(sqlTables)
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
	requestWhere := postgres.WhereByRequest(r)
	sqlSchemaTables := statements.SchemaTables
	if requestWhere != "" {
		sqlSchemaTables = fmt.Sprint(
			statements.SchemaTablesSelect,
			statements.SchemaTablesWhere,
			" AND ",
			requestWhere,
			statements.SchemaTablesOrderBy)
	}

	object, err := postgres.Query(sqlSchemaTables, database, schema)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(object)
}
