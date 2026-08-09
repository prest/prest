package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gotemplate "text/template"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/template"

	"log/slog"
)

func queriesBasePath(cfg *config.Prest) string {
	base := cfg.QueriesPath
	if env := os.Getenv("PREST_QUERIES_LOCATION"); env != "" {
		base = env
	}
	return base
}

// ResolveScript loads SQL template content from the configured storage backend.
func (adapter *postgres) ResolveScript(ctx context.Context, verb, location, name, database string) (adapters.ScriptSource, error) {
	if adapter.cfg.QueriesConf.Storage == config.QueriesStorageDatabase {
		return adapter.resolveScriptDatabase(ctx, verb, location, name, database)
	}
	path, err := adapter.getScriptPath(verb, location, name)
	if err != nil {
		return adapters.ScriptSource{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		slog.Error("could not load script", "script", path, "err", err)
		return adapters.ScriptSource{}, fmt.Errorf("could not load script: %w", err)
	}
	return adapters.ScriptSource{Name: filepath.Base(path), Content: string(content)}, nil
}

func (adapter *postgres) resolveScriptDatabase(ctx context.Context, verb, location, name, database string) (adapters.ScriptSource, error) {
	col, err := scriptVerbColumn(verb)
	if err != nil {
		return adapters.ScriptSource{}, err
	}

	qTable, err := adapter.qualifiedQueriesTable()
	if err != nil {
		return adapters.ScriptSource{}, err
	}

	query := fmt.Sprintf(
		`SELECT %s FROM %s WHERE database_alias = $1 AND location = $2 AND name = $3`, col, qTable)

	db, err := adapter.dbFromCtx(ctx)
	if err != nil {
		return adapters.ScriptSource{}, err
	}

	var content sql.NullString
	var lastErr error
	for _, alias := range queryLookupAliases(database) {
		err = db.QueryRowContext(ctx, query, alias, location, name).Scan(&content)
		if err == nil {
			lastErr = nil
			break
		}
		if err != sql.ErrNoRows {
			return adapters.ScriptSource{}, fmt.Errorf("could not load script: %w", err)
		}
		lastErr = err
	}
	if lastErr != nil {
		return adapters.ScriptSource{}, fmt.Errorf("could not load script: query not found")
	}
	if !content.Valid || content.String == "" {
		return adapters.ScriptSource{}, fmt.Errorf("could not load script: no %s template", verb)
	}
	return adapters.ScriptSource{
		Name:    fmt.Sprintf("%s/%s", location, name),
		Content: content.String,
	}, nil
}

// ParseScriptTemplate renders a SQL template string.
func (adapter *postgres) ParseScriptTemplate(name, content string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error) {
	funcs := template.NewFuncRegistry(templateData)
	tpl := gotemplate.New(name).Funcs(funcs.RegistryAllFuncs())

	tpl, err = tpl.Parse(content)
	if err != nil {
		slog.Error("could not parse template", "name", name, "err", err)
		return "", nil, fmt.Errorf("could not parse template: %w", err)
	}

	var buff bytes.Buffer
	err = tpl.Execute(&buff, funcs.TemplateData)
	if err != nil {
		return "", nil, fmt.Errorf("could not execute template: %w", err)
	}
	return buff.String(), funcs.Args, nil
}

// GetScript get SQL template file path (filesystem mode).
func (adapter *postgres) GetScript(verb, folder, scriptName string) (script string, err error) {
	return adapter.getScriptPath(verb, folder, scriptName)
}

// withinBase reports whether path lies inside base. Both are expected to be
// cleaned already; a path that escapes yields a relative path of ".." or one
// rooted at "..", and an unrelated volume yields an error from filepath.Rel.
// This is a lexical check only — see resolvedWithinBase for symlinks.
func withinBase(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolvedWithinBase repeats the containment check with symlinks resolved, so a
// link inside the queries directory cannot point outside it. Both paths must
// already exist: EvalSymlinks fails on a missing path, which is why callers run
// this after confirming the file is there rather than before.
func resolvedWithinBase(base, path string) bool {
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return false
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return withinBase(filepath.Clean(realBase), filepath.Clean(realPath))
}

func (adapter *postgres) getScriptPath(verb, folder, scriptName string) (string, error) {
	suffix, ok := scriptVerbSuffixes[verb]
	if !ok {
		return "", fmt.Errorf("invalid http method %s", verb)
	}

	base := filepath.Clean(queriesBasePath(adapter.cfg))
	script := filepath.Join(base, folder, fmt.Sprint(scriptName, suffix))

	// folder and scriptName come from the request path. Controllers gate them
	// through validatePathSegments, but the containment check belongs here too so
	// the adapter cannot be made to read outside the queries directory by a caller
	// that skips that gate. filepath.Join has already normalized any "..", so a
	// path that escapes the base is visible as a relative path starting with "..".
	if !withinBase(base, script) {
		return "", fmt.Errorf("invalid script path: %s/%s", folder, scriptName)
	}

	if _, err := os.Stat(script); os.IsNotExist(err) {
		slog.Error("could not load script", "script", script)
		return "", fmt.Errorf("could not load script: %w", err)
	}

	// The file exists, so links can now be resolved: a symlink inside the queries
	// directory that targets a file outside it passes the lexical check above.
	if !resolvedWithinBase(base, script) {
		return "", fmt.Errorf("invalid script path: %s/%s", folder, scriptName)
	}
	return script, nil
}

// ParseScript use values sent by users and add on script file.
func (adapter *postgres) ParseScript(scriptPath string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error) {
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", nil, fmt.Errorf("could not read script: %w", err)
	}
	_, tplName := filepath.Split(scriptPath)
	return adapter.ParseScriptTemplate(tplName, string(content), templateData)
}
