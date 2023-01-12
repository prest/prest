package controllers

import (
	"net/http"

	"github.com/prest/prest/adapters/postgres"
)

func CheckDBHealth() error {
	conn, err := postgres.Get()
	if err != nil {
		return err
	}
	_, err = conn.Exec(";")
	if err != nil {
		return err
	}
	return nil
}

func WrappedHealthCheck(checkDBhealth func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := checkDBhealth(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
