package controllers

import (
	"fmt"
	"net/http"

	"github.com/prest/prest/v2/adapters"

	"github.com/gorilla/mux"
)

// ScriptHandler serves user-defined SQL script endpoints.
type ScriptHandler struct {
	scripts  adapters.ScriptRunner
	executor adapters.QueryExecutor
	db       adapters.DatabaseRegistry
	cache    ResponseCacher
	pgDB     string
}

// NewScriptHandler creates a ScriptHandler.
func NewScriptHandler(deps Deps) *ScriptHandler {
	return &ScriptHandler{
		scripts:  deps.Scripts,
		executor: deps.Executor,
		db:       deps.DB,
		cache:    deps.Cache,
		pgDB:     deps.PGDatabase,
	}
}

// Execute runs a script from the configured queries location.
func (h *ScriptHandler) Execute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queriesPath := vars["queriesLocation"]
	script := vars["script"]
	database := vars["database"]

	if database == "" {
		database = h.db.GetDatabase()
	}

	ctx, cancel := requestContext(r, database)
	defer cancel()

	result, err := h.ExecuteScriptQuery(r.WithContext(ctx), queriesPath, script)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Method == "GET" && h.cache != nil {
		h.cache.BuntSet(r.URL.String(), string(result))
	}
	//nolint
	w.Write(result)
}

// ExecuteScriptQuery runs a script and returns the result bytes.
func (h *ScriptHandler) ExecuteScriptQuery(rq *http.Request, queriesPath string, script string) ([]byte, error) {
	h.db.SetDatabase(h.pgDB)
	sqlPath, err := h.scripts.GetScript(rq.Method, queriesPath, script)
	if err != nil {
		err = fmt.Errorf("could not get script %s/%s, %v", queriesPath, script, err)
		return nil, err
	}

	templateData := make(map[string]interface{})
	extractHeaders(rq, templateData)
	extractQueryParameters(rq, templateData)

	sql, values, err := h.scripts.ParseScript(sqlPath, templateData)
	if err != nil {
		err = fmt.Errorf("could not parse script %s/%s, %v", queriesPath, script, err)
		return nil, err
	}

	sc := h.executor.ExecuteScriptsCtx(rq.Context(), rq.Method, sql, values)
	if sc.Err() != nil {
		err = fmt.Errorf("could not execute sql, check your prest logs")
		return nil, err
	}

	return sc.Bytes(), nil
}

// extractHeaders gets from the given request the headers and populate the provided templateData accordingly.
func extractHeaders(rq *http.Request, templateData map[string]interface{}) {
	headers := map[string]interface{}{}

	for key, value := range rq.Header {
		if len(value) == 1 {
			headers[key] = value[0]
			continue
		}
		headers[key] = value
	}

	templateData["header"] = headers
}

// extractQueryParameters gets from the given request the query parameters and populate the provided templateData
// accordingly.
func extractQueryParameters(rq *http.Request, templateData map[string]interface{}) {
	for key, value := range rq.URL.Query() {
		if len(value) == 1 {
			templateData[key] = value[0]
			continue
		}
		templateData[key] = value
	}
}
