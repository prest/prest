package controllers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	slog "github.com/structy/log"

	pctx "github.com/prest/prest/context"
)

var (
	ErrCouldNotGetScript     = fmt.Errorf("could not get script")
	ErrCouldNotParseScript   = fmt.Errorf("could not parse script")
	ErrCouldNotExecuteScript = fmt.Errorf("could not execute script")
)

// ExecuteScriptQuery is a function to execute and return result of script query
func (c *Config) ExecuteScriptQuery(rq *http.Request, queriesPath string, script string) ([]byte, error) {
	c.adapter.SetCurrentConnDatabase(c.server.PGDatabase)
	sqlPath, err := c.adapter.GetScript(rq.Method, queriesPath, script)
	if err != nil {
		slog.Errorln("could not get script", queriesPath, script, err)
		return nil, ErrCouldNotGetScript
	}

	templateData := make(map[string]interface{})
	extractHeaders(rq, templateData)
	extractQueryParameters(rq, templateData)

	sql, values, err := c.adapter.ParseScript(sqlPath, templateData)
	if err != nil {
		slog.Errorln("could not parse script", queriesPath, script, err)
		return nil, ErrCouldNotParseScript
	}

	sc := c.adapter.ExecuteScriptsCtx(rq.Context(), rq.Method, sql, values)
	if sc.Err() != nil {
		slog.Errorln("could not execute script", queriesPath, script, err)
		return nil, ErrCouldNotExecuteScript
	}

	return sc.Bytes(), nil
}

// ExecuteFromScripts is a controller to peform SQL in scripts created by users
func (c *Config) ExecuteFromScripts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queriesPath := vars["queriesLocation"]
	script := vars["script"]
	database := vars["database"]

	if database == "" {
		database = c.adapter.GetCurrentConnDatabase()
	}

	ctx, cancel := pctx.WithTimeout(
		context.WithValue(r.Context(), pctx.DBNameKey, database))
	defer cancel()

	result, err := c.ExecuteScriptQuery(r.WithContext(ctx), queriesPath, script)
	if err != nil {
		slog.Errorln("execute script error")
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Method == "GET" {
		// Cache arrow if enabled
		c.cache.Set(r.URL.String(), string(result))
	}

	slog.Debugln("execute script success")
	JSONWrite(w, string(result), http.StatusOK)
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
