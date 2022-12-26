package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/adapters/postgres"
)

type HealthCheck struct {
	Status string `json:"status"`
}

// Created an interface for future mocking test
type iDbConnection interface {
	GetConnection() (*sqlx.DB, error)
	RunTestQuery() error
}

type DbConnection struct{}

func (d DbConnection) GetConnection() (db *sqlx.DB, err error) {
	db, err = postgres.Get()
	if err != nil {
		return
	}
	return
}

func (d DbConnection) RunTestQuery() (err error) {
	db, _ := d.GetConnection()
	_, err = db.Exec("SELECT 1")

	if err != nil {
		return err
	}
	return err
}

var dbConn iDbConnection

func init() {
	dbConn = DbConnection{}
}

// HealthStatus - returns 200 if server is fine, else 424 if Postgres not working
func HealthStatus(w http.ResponseWriter, r *http.Request) {
	data := HealthCheck{
		Status: "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	dbc := dbConn
	err := dbc.RunTestQuery()

	if err != nil {
		http.Error(w, "failed to connect", http.StatusFailedDependency)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}
