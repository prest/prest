package controllers

import (
	"log"
	"net/http"

	"github.com/nuveo/prest/adapters/postgres"
	"github.com/nuveo/prest/statements"
)

// GetSchemas list all (or filter) schemas
func GetSchemas(w http.ResponseWriter, r *http.Request) {
	object, err := postgres.Query(statements.Schemas)
	if err != nil {
		log.Println(err)
	}

	w.Write(object)
}
