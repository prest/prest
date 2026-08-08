package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/middlewares"

	"github.com/gorilla/mux"
)

// Reserved templateData keys holding the unscreened values that the `sqlVal` /
// `sqlList` helpers bind. They sit beside the existing `header` key rather than
// widening the ScriptRunner port, and are namespaced with a leading underscore
// so an ordinary query parameter is unlikely to collide.
const (
	rawParamKey  = "_param"
	rawHeaderKey = "_header"
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
//
// A rejected parameter only fails the request if the template interpolated it
// into the SQL text. Had it been passed to sqlVal instead, it would be bound
// and safe, so there is nothing to reject.
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
	rejected := extractQueryParameters(rq, templateData)

	sql, values, err := h.scripts.ParseScriptTemplate(source.Name, source.Content, templateData)
	if err != nil {
		err = fmt.Errorf("could not parse script %s/%s, %v", queriesPath, script, err)
		return nil, err
	}

	if err := rejected.err(); err != nil {
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
//
// Unlike a query parameter, a rejected header does not fail the request: every
// inbound header is screened here, not only the ones a template reads, and an
// ordinary `User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)` fails
// the character allow-list on `(` and `;`. Erroring would reject nearly every
// browser request to every script endpoint. Rejected headers are blanked and
// logged instead.
func extractHeaders(rq *http.Request, templateData map[string]interface{}) {
	headers := map[string]interface{}{}
	raw := map[string]interface{}{}

	for key, value := range rq.Header {
		// Credentials are withheld from the raw map too: blanking them is about
		// secrecy, not SQL composition, so binding must not become a way around it.
		if isCredentialHeader(key) {
			headers[key] = ""
			raw[key] = ""
			continue
		}
		if len(value) == 1 {
			sanitized := sanitizeScriptParam(value[0])
			if sanitized == "" && value[0] != "" {
				warnRejectedHeader(key)
			}
			headers[key] = sanitized
			raw[key] = value[0]
			continue
		}
		sanitized := make([]string, 0, len(value))
		for _, v := range value {
			s := sanitizeScriptParam(v)
			if s == "" && v != "" {
				warnRejectedHeader(key)
			}
			sanitized = append(sanitized, s)
		}
		headers[key] = sanitized
		raw[key] = append([]string(nil), value...)
	}

	templateData["header"] = headers
	templateData[rawHeaderKey] = raw
}

// warnRejectedHeader records that a header was blanked. The name is logged, never
// the value: it is caller-controlled and may carry a credential.
func warnRejectedHeader(key string) {
	slog.Warn("script header value rejected by the SQL safety screen and blanked",
		"header", key,
		"hint", "bind free-form values with the sqlVal template helper")
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
// This is used to prevent SQL injection. A rejected value comes back empty.
//
// The character allow-list alone is not enough: it keeps quotes, commas and
// parentheses out, but letters, digits and space are already sufficient to
// compose a read-only injection such as
// `0 UNION SELECT users::text FROM users` whenever a template interpolates the
// value in an unquoted context (`WHERE id = {{.id}}`). SQL comment (`--`) and
// cast (`::`) tokens are therefore rejected as well.
//
// The SQL keyword screen, however, runs only on values that contain a space.
// Composing SQL takes more than one token, and space is the only separator the
// allow-list permits — `;` `,` `(` `)` `*` `'` `"`, tab and newline are all
// rejected above, and `/**/` is unreachable without `*`. A single-token value is
// therefore inert: `WHERE 1 = teste-do-abc` is a syntax error, not an injection,
// exactly as `WHERE 1 = some_column` already is today. Screening single tokens
// only blanked legitimate data — Portuguese slugs containing `do` were 17.5% of
// one reporter's catalog (issue #1030).
//
// Templates that need free-form values (search phrases, anything with spaces)
// should use the `sqlVal` / `sqlList` helpers, which bind them as query
// parameters and skip this screen entirely.
func sanitizeScriptParam(value string) string {
	if !safeScriptParamRegex.MatchString(value) {
		return ""
	}
	if strings.Contains(value, "--") || strings.Contains(value, "::") {
		return ""
	}
	if !strings.Contains(value, " ") {
		return value
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
//
// A value the sanitizer rejects fails the request. Blanking it and carrying on
// used to leave `{{if isSet "x"}}` true and run the query with `”`, so the
// caller got HTTP 200 and rows belonging to a different record — indistinguishable
// from a genuine miss (issue #1030).
//
// Raw, unscreened values are also published under rawParamKey for the `sqlVal` /
// `sqlList` helpers, which bind them instead of interpolating them.
func extractQueryParameters(rq *http.Request, templateData map[string]interface{}) *rejectedUsage {
	usage := &rejectedUsage{}
	raw := map[string]interface{}{}
	for key, value := range rq.URL.Query() {
		if len(value) == 1 {
			templateData[key] = screenedValue(key, value[0], usage)
			raw[key] = value[0]
			continue
		}
		// Stay a []string while every element survives the screen: inFormat and
		// sqlList both type-assert on it. Only a rejection forces the wider type,
		// so that the marker can report itself when rendered.
		sanitized := make([]string, 0, len(value))
		rejectedAt := -1
		for i, v := range value {
			s := sanitizeScriptParam(v)
			if s == "" && v != "" {
				rejectedAt = i
			}
			sanitized = append(sanitized, s)
		}
		if rejectedAt >= 0 {
			mixed := make([]interface{}, 0, len(value))
			for _, v := range value {
				mixed = append(mixed, screenedValue(key, v, usage))
			}
			templateData[key] = mixed
		} else {
			templateData[key] = sanitized
		}
		raw[key] = append([]string(nil), value...)
	}
	templateData[rawParamKey] = raw
	return usage
}

// screenedValue returns the value to interpolate inline: the sanitized string
// when it survives the screen, otherwise a marker that reports itself if the
// template actually renders it.
func screenedValue(key, value string, usage *rejectedUsage) interface{} {
	sanitized := sanitizeScriptParam(value)
	if sanitized == "" && value != "" {
		return rejectedParam{key: key, usage: usage}
	}
	return sanitized
}

// rejectedParam stands in for a query parameter the safety screen refused.
//
// The decision to fail cannot be taken when the parameter is read, because a
// template may pass it to `sqlVal`, which binds it as a query parameter where SQL
// composition is impossible and no screening is warranted. So the marker renders
// as the empty string exactly like before, and records that it was interpolated
// into the SQL text. The handler fails the request afterwards only if that
// happened — turning the silent wrong-rows response of issue #1030 into an error,
// without rejecting values that were only ever bound.
type rejectedParam struct {
	key   string
	usage *rejectedUsage
}

// String satisfies fmt.Stringer, which is how text/template renders the value.
func (r rejectedParam) String() string {
	r.usage.keys = append(r.usage.keys, r.key)
	return ""
}

// rejectedUsage collects the parameters whose rejected value reached the SQL text.
type rejectedUsage struct {
	keys []string
}

// err names the first offending parameter, without echoing its value: the value
// is caller-controlled, may carry a credential, and would end up in logs
// (CodeQL go/clear-text-logging).
func (u *rejectedUsage) err() error {
	if len(u.keys) == 0 {
		return nil
	}
	return fmt.Errorf(
		"invalid value for parameter %s: it contains SQL syntax that cannot be interpolated safely; "+
			"use the sqlVal template helper to bind free-form values", u.keys[0])
}
