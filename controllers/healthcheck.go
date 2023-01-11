package controllers

//go:generate mockgen -source=healthcheck.go -destination=./mocks/healthcheck.go -package=mocks

import (
	"encoding/json"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/prest/prest/adapters/postgres"
)

type HealthCheck struct {
	Status string `json:"status"`
}

type DbConnection interface {
	GetConnection() (*sqlx.DB, error)
	RunTestQuery() error
}

type DBConn struct{}

func (d DBConn) GetConnection() (db *sqlx.DB, err error) {
	db, err = postgres.Get()
	if err != nil {
		return
	}
	return
}

func (d DBConn) RunTestQuery() (err error) {
	db, err := d.GetConnection()
	if err != nil {
		return err
	}

	_, err = db.Exec(";")

	return err
}

func WrappedHealthCheck(dbc DbConnection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		err := dbc.RunTestQuery()

		if err != nil {
			http.Error(w, "unable to run queries on the database", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(HealthCheck{
			Status: "ok",
		})
	}
}
