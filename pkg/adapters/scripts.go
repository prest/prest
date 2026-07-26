package adapters

import "context"

// ScriptSource holds resolved SQL template content.
type ScriptSource struct {
	Name    string
	Content string
}

// ScriptRunner loads and parses user-defined SQL scripts.
type ScriptRunner interface {
	ResolveScript(ctx context.Context, verb, location, name, database string) (ScriptSource, error)
	ParseScriptTemplate(name, content string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error)

	// Deprecated: use ResolveScript + ParseScriptTemplate.
	GetScript(verb, folder, scriptName string) (script string, err error)
	// Deprecated: use ParseScriptTemplate.
	ParseScript(scriptPath string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error)
}
