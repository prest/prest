package adapters

// ScriptRunner loads and parses user-defined SQL scripts.
type ScriptRunner interface {
	GetScript(verb, folder, scriptName string) (script string, err error)
	ParseScript(scriptPath string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error)
}
