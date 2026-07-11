package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	funcs := &template.FuncRegistry{TemplateData: templateData}
	tpl := gotemplate.New(name).Funcs(funcs.RegistryAllFuncs())

	tpl, err = tpl.Parse(content)
	if err != nil {
		slog.Error("could not parse template", "name", name, "err", err)
		return "", nil, fmt.Errorf("could not parse template: %w", err)
	}

	var buff bytes.Buffer
	err = tpl.Execute(&buff, funcs.TemplateData)
	if err != nil {
		return "", nil, fmt.Errorf("could not execute template %v", err)
	}
	return buff.String(), funcs.Args, nil
}

// GetScript get SQL template file path (filesystem mode).
func (adapter *postgres) GetScript(verb, folder, scriptName string) (script string, err error) {
	return adapter.getScriptPath(verb, folder, scriptName)
}

func (adapter *postgres) getScriptPath(verb, folder, scriptName string) (string, error) {
	suffix, ok := scriptVerbSuffixes[verb]
	if !ok {
		return "", fmt.Errorf("invalid http method %s", verb)
	}

	base := queriesBasePath(adapter.cfg)
	script := filepath.Join(base, folder, fmt.Sprint(scriptName, suffix))

	if _, err := os.Stat(script); os.IsNotExist(err) {
		slog.Error("could not load script", "script", script)
		return "", fmt.Errorf("could not load script: %w", err)
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
