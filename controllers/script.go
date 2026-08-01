package controllers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/middlewares"

	"github.com/gorilla/mux"
)

// ScriptHandler serves user-defined SQL script endpoints.
type ScriptHandler struct {
	scripts  adapters.ScriptRunner
	executor adapters.QueryExecutor
	db       adapters.DatabaseRegistry
	cache    ResponseCacher
	pgDB     string
	singleDB bool
}

// NewScriptHandler creates a ScriptHandler.
func NewScriptHandler(deps Deps) *ScriptHandler {
	return &ScriptHandler{
		scripts:  deps.Scripts,
		executor: deps.Executor,
		db:       deps.DB,
		cache:    deps.Cache,
		pgDB:     deps.PGDatabase,
		singleDB: deps.SingleDB,
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

	// Same gate every other controller applies before touching the connection
	// layer: reject an unregistered/attacker-chosen database name here, rather
	// than letting dbFromCtx open a real outbound connection for it.
	if err := validateDatabase(database, h.db, h.singleDB); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !validatePathSegments(database, queriesPath, script) {
		jsonError(w, "invalid identifier in path", http.StatusBadRequest)
		return
	}

	ctx, cancel := requestContext(r, database)
	defer cancel()

	result, err := h.ExecuteScriptQuery(r.WithContext(ctx), queriesPath, script)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Method == "GET" && h.cache != nil {
		h.cache.BuntSet(middlewares.CacheKey(r), string(result))
	}
	//nolint
	w.Write(result)
}

// ExecuteScriptQuery runs a script and returns the result bytes.
func (h *ScriptHandler) ExecuteScriptQuery(rq *http.Request, queriesPath string, script string) ([]byte, error) {
	vars := mux.Vars(rq)
	database := vars["database"] // empty = default prest_queries.database_alias

	source, err := h.scripts.ResolveScript(rq.Context(), rq.Method, queriesPath, script, database)
	if err != nil {
		err = fmt.Errorf("could not get script %s/%s, %v", queriesPath, script, err)
		return nil, err
	}

	templateData := make(map[string]interface{})
	extractHeaders(rq, templateData)
	extractQueryParameters(rq, templateData)

	sql, values, err := h.scripts.ParseScriptTemplate(source.Name, source.Content, templateData)
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

// isCredentialHeader reports whether a header carries a caller credential, which
// must never reach a script template with its real value.
// sanitizeScriptParam screens for SQL composition, not for secrecy: a bearer
// token or session cookie is plain base64url text and passes its allow-list
// untouched, so a template referencing one would interpolate the caller's
// credential into the SQL string that adapters log at debug level. The key is
// kept with an empty value — the same shape sanitizeScriptParam gives a rejected
// value — so an existing template still renders a valid empty literal instead of
// Go's "<no value>". Keys are in net/http canonical form, which is how they are
// stored in Request.Header.
func isCredentialHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Authorization", "Proxy-Authorization", "Cookie",
		"X-Api-Key", "X-Auth-Token", "X-Access-Token":
		return true
	default:
		return false
	}
}

// extractHeaders gets from the given request the headers and populate the provided templateData accordingly.
// Values are routed through sanitizeScriptParam, the same gate used for query
// parameters, since templates interpolate headers into SQL exactly the same way.
// Credential-bearing headers are dropped entirely rather than sanitized.
func extractHeaders(rq *http.Request, templateData map[string]interface{}) {
	headers := map[string]interface{}{}

	for key, value := range rq.Header {
		if isCredentialHeader(key) {
			headers[key] = ""
			continue
		}
		if len(value) == 1 {
			headers[key] = sanitizeScriptParam(value[0])
			continue
		}
		sanitized := make([]string, 0, len(value))
		for _, v := range value {
			sanitized = append(sanitized, sanitizeScriptParam(v))
		}
		headers[key] = sanitized
	}

	templateData["header"] = headers
}

var safeScriptParamRegex = regexp.MustCompile(`^[a-zA-Z0-9_.:@/\\ -]+$`)

// scriptParamWordRegex extracts the word tokens of a value so they can be
// screened against SQL keywords. Tokens are whole alphanumeric runs, so
// `order66` or `table9` stay intact instead of matching on an `order`/`table`
// prefix, while `0-union` and `a/select` still split on the separator and get
// caught.
var scriptParamWordRegex = regexp.MustCompile(`[a-zA-Z0-9_]+`)

// scriptParamLeadingDigits matches the numeric prefix of a token. PostgreSQL
// before v15 lexes `0union` as `0` followed by the keyword, so the digits are
// stripped before the keyword lookup; `order66` has no numeric prefix and is
// unaffected.
var scriptParamLeadingDigits = regexp.MustCompile(`^[0-9]+`)

// scriptParamSQLKeywords are the tokens that make SQL composition possible once
// a value lands in an unquoted template context. Values carrying any of them
// are rejected outright.
var scriptParamSQLKeywords = map[string]struct{}{
	"all": {}, "alter": {}, "and": {}, "any": {}, "as": {}, "between": {},
	"by": {}, "call": {}, "case": {}, "cast": {}, "copy": {}, "create": {},
	"database": {}, "delete": {}, "distinct": {}, "do": {}, "drop": {},
	"except": {}, "exec": {}, "execute": {}, "exists": {}, "fetch": {},
	"from": {}, "grant": {}, "group": {}, "having": {}, "ilike": {},
	"insert": {}, "intersect": {}, "into": {}, "join": {}, "lateral": {},
	"like": {}, "limit": {}, "not": {}, "null": {}, "offset": {}, "only": {},
	"or": {}, "order": {}, "over": {}, "returning": {}, "revoke": {},
	"schema": {}, "select": {}, "set": {}, "similar": {}, "table": {},
	"then": {}, "truncate": {}, "union": {}, "update": {}, "using": {},
	"values": {}, "when": {}, "where": {}, "window": {}, "with": {},
}

// sanitizeScriptParam sanitizes the given value to be used as a script parameter.
// This is used to prevent SQL injection.
//
// The character allow-list alone is not enough: it keeps quotes, commas and
// parentheses out, but letters, digits and space are already sufficient to
// compose a read-only injection such as
// `0 UNION SELECT users::text FROM users` whenever a template interpolates the
// value in an unquoted context (`WHERE id = {{.id}}`). SQL comment (`--`) and
// cast (`::`) tokens plus any SQL keyword are therefore rejected as well.
//
// Templates that need to interpolate free-form values should use the `sqlVal`
// / `sqlList` helpers, which bind them as query parameters instead.
func sanitizeScriptParam(value string) string {
	if !safeScriptParamRegex.MatchString(value) {
		return ""
	}
	if strings.Contains(value, "--") || strings.Contains(value, "::") {
		return ""
	}
	for _, word := range scriptParamWordRegex.FindAllString(value, -1) {
		word = strings.ToLower(word)
		if _, ok := scriptParamSQLKeywords[word]; ok {
			return ""
		}
		if _, ok := scriptParamSQLKeywords[scriptParamLeadingDigits.ReplaceAllString(word, "")]; ok {
			return ""
		}
	}
	return value
}

// extractQueryParameters gets from the given request the query parameters and populate the provided templateData
// accordingly.
func extractQueryParameters(rq *http.Request, templateData map[string]interface{}) {
	for key, value := range rq.URL.Query() {
		if len(value) == 1 {
			templateData[key] = sanitizeScriptParam(value[0])
			continue
		}
		sanitized := make([]string, 0, len(value))
		for _, v := range value {
			sanitized = append(sanitized, sanitizeScriptParam(v))
		}
		templateData[key] = sanitized
	}
}
