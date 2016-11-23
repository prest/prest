package controllers

import (
	"log"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetTables list all (or filter) tables
func GetTables(w http.ResponseWriter, r *http.Request) {
	object, err := postgres.Query(statements.Tables)
	if err != nil {
		log.Println(err)
	}

	w.Write(object)
}
