package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/middlewares"

	"github.com/gorilla/mux"
)

const maxQueryRegistryBody = 1 << 20 // 1 MiB

// QueryRegistryHandler manages prest_queries via HTTP.
type QueryRegistryHandler struct {
	registry adapters.QueryRegistry
	db       adapters.DatabaseRegistry
	cfg      config.QueriesConf
}

// NewQueryRegistryHandler creates a QueryRegistryHandler.
func NewQueryRegistryHandler(deps Deps, cfg config.QueriesConf) *QueryRegistryHandler {
	return &QueryRegistryHandler{
		registry: deps.QueryRegistry,
		db:       deps.DB,
		cfg:      cfg,
	}
}

// List handles GET /_QUERIES/registry.
func (h *QueryRegistryHandler) List(w http.ResponseWriter, r *http.Request) {
	database := r.URL.Query().Get("database")
	location := r.URL.Query().Get("location")

	ctx, cancel := requestContext(r, h.db.GetDatabase())
	defer cancel()

	queries, err := h.registry.ListQueries(ctx, database, location)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, queries)
}

// Get handles GET /_QUERIES/registry/{location}/{name}.
func (h *QueryRegistryHandler) Get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	if database == "" {
		database = r.URL.Query().Get("database")
	}

	ctx, cancel := requestContext(r, h.db.GetDatabase())
	defer cancel()

	q, err := h.registry.GetQuery(ctx, database, vars["location"], vars["name"])
	if err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, q)
}

// Create handles POST /_QUERIES/registry.
func (h *QueryRegistryHandler) Create(w http.ResponseWriter, r *http.Request) {
	q, err := h.decodeBody(w, r)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	q.CreatedBy = middlewares.AdminUsernameFromContext(r.Context())

	ctx, cancel := requestContext(r, h.db.GetDatabase())
	defer cancel()

	if err := h.registry.UpsertQuery(ctx, q); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, q)
}

// Update handles PUT /_QUERIES/registry/{location}/{name}.
func (h *QueryRegistryHandler) Update(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	q, err := h.decodeBody(w, r)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	q.Location = vars["location"]
	q.Name = vars["name"]
	if vars["database"] != "" {
		q.DatabaseAlias = vars["database"]
	}
	q.CreatedBy = middlewares.AdminUsernameFromContext(r.Context())

	ctx, cancel := requestContext(r, h.db.GetDatabase())
	defer cancel()

	if err := h.registry.UpsertQuery(ctx, q); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, q)
}

// Delete handles DELETE /_QUERIES/registry/{location}/{name}.
func (h *QueryRegistryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	database := vars["database"]
	if database == "" {
		database = r.URL.Query().Get("database")
	}

	ctx, cancel := requestContext(r, h.db.GetDatabase())
	defer cancel()

	if err := h.registry.DeleteQuery(ctx, database, vars["location"], vars["name"]); err != nil {
		jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QueryRegistryHandler) decodeBody(w http.ResponseWriter, r *http.Request) (adapters.StoredQuery, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxQueryRegistryBody)
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return adapters.StoredQuery{}, fmt.Errorf("invalid request body")
	}
	var q adapters.StoredQuery
	if err := json.Unmarshal(body, &q); err != nil {
		return adapters.StoredQuery{}, fmt.Errorf("invalid json body")
	}
	return q, nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
