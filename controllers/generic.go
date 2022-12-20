package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/prest/prest/adapters/postgres"
)

type HealthCheck struct {
	Status string `json:"status"`
}

// GetHealthStatus - returns 200 if server is fine, else 424 if Postgres not working
func GetHealthStatus(w http.ResponseWriter, r *http.Request) {
	data := HealthCheck{
		Status: "ok",
	}

	w.Header().Set("Content-Type", "application/json")

	db, err := postgres.Get()

	if err != nil {
		http.Error(w, "failed to connect", http.StatusFailedDependency)
		return
	}

	_, err = db.Exec("SELECT 1")

	if err != nil {
		http.Error(w, "failed to query", http.StatusFailedDependency)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
	return
}
