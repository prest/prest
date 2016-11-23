package controllers

import (
	"log"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetDatabases list all (or filter) databases
func GetTables(w http.ResponseWriter, r *http.Request) {
	object, err := postgres.Query(statements.Tables)
	if err != nil {
		log.Println(err)
	}

	w.Write(object)
}
