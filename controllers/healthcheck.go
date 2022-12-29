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

// iDbConnection is the interface of the health test database context
// used externally (e.g. in the view or even in tests)
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
	_, err = db.Exec(";")

	if err != nil {
		return err
	}
	return err
}

var dbConn iDbConnection

func init() {
	dbConn = DbConnection{}
}

// HealthStatus - returns 200 if server is fine, else 503 if Postgres not working
func HealthStatus(w http.ResponseWriter, r *http.Request) {
	data := HealthCheck{
		Status: "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	dbc := dbConn
	err := dbc.RunTestQuery()

	if err != nil {
		http.Error(w, "failed to connect", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}
