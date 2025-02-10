package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	gotemplate "text/template"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
	"github.com/prest/prest/v2/adapters/scanner"
	"github.com/prest/prest/v2/config"
	"github.com/prest/prest/v2/template"

	"github.com/structy/log"
)

// GetScript get SQL template file
func (adapter *Postgres) GetScript(verb, folder, scriptName string) (script string, err error) {
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

	script = filepath.Join(config.PrestConf.QueriesPath, folder, fmt.Sprint(scriptName, sufix))

	if _, err = os.Stat(script); os.IsNotExist(err) {
		err = fmt.Errorf("could not load %s", script)
		return
	}

	return
}

// ParseScript use values sent by users and add on script
func (adapter *Postgres) ParseScript(scriptPath string, templateData map[string]interface{}) (sqlQuery string, values []interface{}, err error) {
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
func WriteSQL(sql string, values []interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	stmt, err := Prepare(db, sql)
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
func WriteSQLCtx(ctx context.Context, sql string, values []interface{}) (sc adapters.Scanner) {
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	stmt, err := Prepare(db, sql)
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
func (adapter *Postgres) ExecuteScripts(method, sql string, values []interface{}) (sc adapters.Scanner) {
	switch method {
	case "GET":
		return adapter.Query(sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return WriteSQL(sql, values)
	}
	return &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
}

// ExecuteScriptsCtx run sql templates created by users
func (adapter *Postgres) ExecuteScriptsCtx(ctx context.Context, method, sql string, values []interface{}) (sc adapters.Scanner) {
	switch method {
	case "GET":
		return adapter.QueryCtx(ctx, sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		return WriteSQLCtx(ctx, sql, values)
	}
	return &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
}
