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

	"github.com/prest/prest/adapters"
	"github.com/prest/prest/adapters/scanner"
	"github.com/prest/prest/template"
)

// GetScript get SQL template file
func (a Adapter) GetScript(verb, folder, scriptName string) (script string, err error) {
	verbs := map[string]string{
		"GET":    ".read.sql",
		"POST":   ".write.sql",
		"PATCH":  ".update.sql",
		"PUT":    ".update.sql",
		"DELETE": ".delete.sql",
	}

	sufix, ok := verbs[verb]
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
func (a Adapter) ParseScript(scriptPath string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error) {
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
func (a Adapter) WriteSQL(sql string, values []interface{}) (sc adapters.Scanner) {
	db, err := a.conn.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	stmt, err := a.Prepare(db, sql, false)
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", sql, err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	valuesAux := make([]interface{}, 0, len(values))
	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	result, err := stmt.Exec(valuesAux...)
	if err != nil {
		log.Printf("sql = %v\n", sql)
		err = fmt.Errorf("could not peform sql: %v", err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("could not rows affected: %v", err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var resultByte []byte
	resultByte, err = json.Marshal(data)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(resultByte),
	}
	return
}

// WriteSQLCtx perform INSERT's, UPDATE's, DELETE's operations
func (a Adapter) WriteSQLCtx(ctx context.Context, sql string, values []interface{}) (sc adapters.Scanner) {
	db, err := a.getDBFromCtx(ctx)
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	stmt, err := a.Prepare(db, sql, false)
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", sql, err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	valuesAux := make([]interface{}, 0, len(values))
	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	result, err := stmt.Exec(valuesAux...)
	if err != nil {
		log.Printf("sql = %v\n", sql)
		err = fmt.Errorf("could not peform sql: %v", err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		err = fmt.Errorf("could not rows affected: %v", err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var resultByte []byte
	resultByte, err = json.Marshal(data)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(resultByte),
	}
	return
}

// ExecuteScripts run sql templates created by users
func (a Adapter) ExecuteScripts(method, sql string, values []interface{}) (sc adapters.Scanner) {
	switch method {
	case "GET":
		return a.Query(sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return a.WriteSQL(sql, values)
	}
	return &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
}

// ExecuteScriptsCtx run sql templates created by users
func (a Adapter) ExecuteScriptsCtx(ctx context.Context, method, sql string, values []interface{}) (sc adapters.Scanner) {
	switch method {
	case "GET":
		return a.QueryCtx(ctx, sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return a.WriteSQLCtx(ctx, sql, values)
	}
	return &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
}
