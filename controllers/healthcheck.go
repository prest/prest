package controllers

//go:generate mockgen -source=healthcheck.go -destination=../mocks/healthcheck.go -package=mocks

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

type SDbConnection struct{}

func (d SDbConnection) GetConnection() (db *sqlx.DB, err error) {
	db, err = postgres.Get()
	if err != nil {
		return
	}
	return
}

func (d SDbConnection) RunTestQuery() (err error) {
	db, err := d.GetConnection()

	if err != nil {
		return err
	}

	_, err = db.Exec(";")

	return err
}

func WrappedHealthCheck(dbc DbConnection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := HealthCheck{
			Status: "ok",
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := dbc.GetConnection()

		if err != nil {
			http.Error(w, "failed to connect", http.StatusServiceUnavailable)
			return
		}

		err = dbc.RunTestQuery()

		if err != nil {
			http.Error(w, "failed to query", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(data)
	}
}
