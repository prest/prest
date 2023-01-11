package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/prest/prest/adapters/postgres"
)

type HealthCheck struct {
	Status string `json:"status"`
}

func CheckDBHealth() error {
	conn, err := postgres.Get()
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Exec(";")
	if err != nil {
		return err
	}
	return nil
}

func WrappedHealthCheck(checkDBhealth func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := checkDBhealth(); err != nil {
			http.Error(w, "unable to run queries on the database", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(HealthCheck{"ok"}); err != nil {
			http.Error(w, "unable to enconde json response", http.StatusServiceUnavailable)
			return
		}
	}
}
