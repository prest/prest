package mysql

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
)

var scriptVerbSuffixes = map[string]string{
	"GET":    ".read.sql",
	"POST":   ".write.sql",
	"PATCH":  ".update.sql",
	"PUT":    ".update.sql",
	"DELETE": ".delete.sql",
}

var scriptSuffixColumns = map[string]string{
	".read.sql":   "read_sql",
	".write.sql":  "write_sql",
	".update.sql": "update_sql",
	".delete.sql": "delete_sql",
}

func (a *Adapter) ResolveScript(ctx context.Context, verb, location, name, database string) (adapters.ScriptSource, error) {
	if a.cfg != nil && a.cfg.QueriesConf.Storage == config.QueriesStorageDatabase {
		return a.resolveScriptDatabase(ctx, verb, location, name, database)
	}
	path, err := a.scriptPath(verb, location, name)
	if err != nil {
		return adapters.ScriptSource{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return adapters.ScriptSource{}, fmt.Errorf("could not load script: %w", err)
	}
	return adapters.ScriptSource{Name: filepath.Base(path), Content: string(content)}, nil
}

func (a *Adapter) resolveScriptDatabase(ctx context.Context, verb, location, name, database string) (adapters.ScriptSource, error) {
	col, err := scriptVerbColumn(verb)
	if err != nil {
		return adapters.ScriptSource{}, err
	}
	qTable, err := a.qualifiedQueriesTable()
	if err != nil {
		return adapters.ScriptSource{}, err
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE database_alias = ? AND location = ? AND name = ?", col, qTable)
	db, err := a.dbFromCtx(ctx)
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
	return adapters.ScriptSource{Name: location + "/" + name, Content: content.String}, nil
}

func scriptVerbColumn(verb string) (string, error) {
	suffix, ok := scriptVerbSuffixes[verb]
	if !ok {
		return "", fmt.Errorf("invalid http method %s", verb)
	}
	col, ok := scriptSuffixColumns[suffix]
	if !ok {
		return "", fmt.Errorf("invalid http method %s", verb)
	}
	return col, nil
}

func queryLookupAliases(database string) []string {
	if database == "" {
		return []string{""}
	}
	return []string{database, ""}
}

func (a *Adapter) ParseScriptTemplate(name, content string, templateData map[string]interface{}) (string, []interface{}, error) {
	funcs := template.NewMySQLFuncRegistry(templateData)
	tpl := gotemplate.New(name).Funcs(funcs.RegistryAllFuncs())
	tpl, err := tpl.Parse(content)
	if err != nil {
		return "", nil, fmt.Errorf("could not parse template: %w", err)
	}
	var buff bytes.Buffer
	if err = tpl.Execute(&buff, funcs.TemplateData); err != nil {
		return "", nil, fmt.Errorf("could not execute template: %w", err)
	}
	return buff.String(), funcs.Args, nil
}

func (a *Adapter) GetScript(verb, folder, scriptName string) (string, error) {
	return a.scriptPath(verb, folder, scriptName)
}

func (a *Adapter) ParseScript(scriptPath string, templateData map[string]interface{}) (string, []interface{}, error) {
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", nil, fmt.Errorf("could not read script: %w", err)
	}
	_, tplName := filepath.Split(scriptPath)
	return a.ParseScriptTemplate(tplName, string(content), templateData)
}

func (a *Adapter) scriptPath(verb, folder, scriptName string) (string, error) {
	suffix, ok := scriptVerbSuffixes[verb]
	if !ok {
		return "", fmt.Errorf("invalid http method %s", verb)
	}
	base := filepath.Clean(queriesBasePath(a.cfg))
	script := filepath.Join(base, folder, scriptName+suffix)
	if !withinBase(base, script) {
		return "", fmt.Errorf("invalid script path: %s/%s", folder, scriptName)
	}
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return "", fmt.Errorf("could not load script: %w", err)
	}
	if !resolvedWithinBase(base, script) {
		return "", fmt.Errorf("invalid script path: %s/%s", folder, scriptName)
	}
	return script, nil
}

func queriesBasePath(cfg *config.Prest) string {
	base := ""
	if cfg != nil {
		base = cfg.QueriesPath
	}
	if env := os.Getenv("PREST_QUERIES_LOCATION"); env != "" {
		base = env
	}
	return base
}

func withinBase(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

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
