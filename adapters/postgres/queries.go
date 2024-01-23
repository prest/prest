package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	gotemplate "text/template"

	"github.com/structy/log"

	"github.com/prest/prest/adapters/scanner"
	"github.com/prest/prest/template"
)

// GetScript gets the SQL template file
func (a adapter) GetScript(verb, folder, scriptName string) (script string, err error) {
	sufix, ok := a.scriptVerbs[verb]
	if !ok {
		err = fmt.Errorf("invalid http method %s", verb)
		return
	}

	script = filepath.Join(a.cfg.QueriesPath, folder, fmt.Sprint(scriptName, sufix))

	if _, err = os.Stat(script); os.IsNotExist(err) {
		err = fmt.Errorf("could not load %s", script)
		return
	}

	return
}

// ParseScript use values sent by users and add on script
func (a adapter) ParseScript(scriptPath string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error) {
	_, tplName := filepath.Split(scriptPath)

	funcs := &template.FuncRegistry{TemplateData: templateData}
	tpl := gotemplate.New(tplName).Funcs(funcs.RegistryAllFuncs())

	tpl, err = tpl.ParseFiles(scriptPath)
	if err != nil {
		err = fmt.Errorf("could not parse file %s: %v", scriptPath, err)
		return
	}

	var buff bytes.Buffer
	err = tpl.Execute(&buff, funcs.TemplateData)
	if err != nil {
		err = fmt.Errorf("could not execute template %v", err)
		return
	}

	sqlQuery = buff.String()
	return
}

// WriteSQL perform INSERT's, UPDATE's, DELETE's operations
func (a adapter) WriteSQL(sql string, values []interface{}) scanner.Scanner {
	db, err := a.pool.Get()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	stmt, err := a.Prepare(db, sql, false)
	if err != nil {
		log.Errorf("could not prepare sql: %s\n Error: %v\n", sql, err)
		return &scanner.PrestScanner{Error: err}
	}

	valuesAux := make([]interface{}, 0, len(values))
	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	result, err := stmt.Exec(valuesAux...)
	if err != nil {
		log.Errorf("sql = %v\n", sql)
		err = fmt.Errorf("could not peform sql: %v", err)
		return &scanner.PrestScanner{Error: err}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("could not rows affected: %v", err)
		return &scanner.PrestScanner{Error: err}
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var resultByte []byte
	resultByte, err = json.Marshal(data)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(resultByte),
	}
}

// WriteSQLCtx perform INSERT's, UPDATE's, DELETE's operations
func (a adapter) WriteSQLCtx(ctx context.Context, sql string, values []interface{}) scanner.Scanner {
	db, err := a.getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	stmt, err := a.Prepare(db, sql, false)
	if err != nil {
		log.Errorf("could not prepare sql: %s\n Error: %v\n", sql, err)
		return &scanner.PrestScanner{Error: err}
	}

	valuesAux := make([]interface{}, 0, len(values))
	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	result, err := stmt.Exec(valuesAux...)
	if err != nil {
		log.Errorf("sql = %v\n", sql)
		err = fmt.Errorf("could not peform sql: %v", err)
		return &scanner.PrestScanner{Error: err}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("could not rows affected: %v", err)
		return &scanner.PrestScanner{Error: err}
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var resultByte []byte
	resultByte, err = json.Marshal(data)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(resultByte),
	}
}

// ExecuteScripts run sql templates created by users
func (a adapter) ExecuteScripts(method, sql string, values []interface{}) (sc scanner.Scanner) {
	switch method {
	case "GET":
		return a.Query(sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return a.WriteSQL(sql, values)
	}
	return &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
}

// ExecuteScriptsCtx run sql templates created by users
func (a adapter) ExecuteScriptsCtx(ctx context.Context, method, sql string, values []interface{}) (sc scanner.Scanner) {
	switch method {
	case "GET":
		return a.QueryCtx(ctx, sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return a.WriteSQLCtx(ctx, sql, values)
	}
	return &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
}
