package controllers

//go:generate mockgen -source=healthcheck.go -destination=./mocks/healthcheck.go -package=mocks

import (
	"encoding/json"
	"net/http"

	"github.com/prest/prest/adapters/postgres"
)

type HealthCheck struct {
	Status string `json:"status"`
}

type DbConnection interface {
	ConnectionTest() error
}

type DBConn struct{}

func (d DBConn) ConnectionTest() (err error) {
	conn, err := postgres.Get()
	if err != nil {
		return err
	}

	_, err = conn.Exec(";")

	if err != nil {
		return err
	}

	defer conn.Close()
	return err
}

func WrappedHealthCheck(dbc DbConnection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		err := dbc.ConnectionTest()

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
