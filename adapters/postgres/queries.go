package postgres

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	gotemplate "text/template"

	"github.com/nuveo/log"
	"github.com/prest/prest/adapters"
	"github.com/prest/prest/adapters/postgres/internal/connection"
	"github.com/prest/prest/adapters/scanner"
	"github.com/prest/prest/config"
	"github.com/prest/prest/template"
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
	_, tplName := path.Split(scriptPath)

	funcs := &template.FuncRegistry{TemplateData: templateData}
	tpl := gotemplate.New(tplName).Funcs(funcs.RegistryAllFuncs())

	tpl, err = tpl.ParseFiles(scriptPath)
	if err != nil {
		err = fmt.Errorf("could not parse file %s: %+v", scriptPath, err)
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
	db, err := connection.Get(config.PrestConf.PGDatabase)
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
		log.Printf("sql = %+v\n", sql)
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
		sc = adapter.Query(config.PrestConf.PGDatabase, sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		sc = WriteSQL(sql, values)
	default:
		sc = &scanner.PrestScanner{Error: fmt.Errorf("invalid method %s", method)}
	}

	return
}
