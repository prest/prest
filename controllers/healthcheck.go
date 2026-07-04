package controllers

import (
	"context"
	"net/http"
	"time"

	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/internal/logsafe"

	"log/slog"
)

// HealthCheckFunc validates a subsystem for the health endpoint.
type HealthCheckFunc func(context.Context) error

// CheckList is a list of health check functions.
type CheckList []HealthCheckFunc

// HealthHandler serves the health check endpoint.
type HealthHandler struct {
	checks CheckList
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(checks CheckList) *HealthHandler {
	return &HealthHandler{checks: checks}
}

// ServeHTTP runs all health checks and returns 200 or 503.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	timeout, _ := r.Context().Value(pctx.HTTPTimeoutKey).(int)
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(timeout))
	defer cancel()
	for _, check := range h.checks {
		if err := check(ctx); err != nil {
			slog.Error("could not check DB connection", "err", logsafe.Error(err))
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// Handler returns an http.HandlerFunc for route registration.
func (h *HealthHandler) Handler() http.HandlerFunc {
	return h.ServeHTTP
}
