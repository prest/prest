package controllers

import (
	"log"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetDatabases list all (or filter) databases
func GetDatabases(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	object, err := postgres.Query(statements.Databases)
	if err != nil {
		log.Println(err)
	}

	w.Write(object)
}
