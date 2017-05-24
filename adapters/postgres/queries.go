package postgres

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"text/template"

	"github.com/nuveo/prest/adapters/postgres/connection"
	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/tpl"
)

// GetScript get SQL template file
func GetScript(verb, folder, scriptName string) (script string, err error) {
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
func ParseScript(scriptPath string, queryURL url.Values) (sqlQuery string, values []interface{}, err error) {
	tmpl, err := template.ParseFiles(scriptPath)
	if err != nil {
		err = fmt.Errorf("could not parse file %s: %+v", scriptPath, err)
		return
	}
	tmpl = tmpl.Option("missingkey=error")

	q := make(map[string]string)
	pid := 1
	for key := range queryURL {
		q[key] = queryURL.Get(key)
		pid++
	}
	funcs := &tpl.TemplateFuncRegistry{TplData: q}
	tmpl = tmpl.Funcs(funcs.AllFuncs())

	var buff bytes.Buffer
	err = tmpl.Execute(&buff, q)
	if err != nil {
		err = fmt.Errorf("could not execute template %v", err)
		return
	}

	sqlQuery = buff.String()
	return
}

// WriteSQL perform INSERT's, UPDATE's, DELETE's operations
func WriteSQL(sql string, values []interface{}) (resultByte []byte, err error) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("could not begin transaction: %v\n", err)
		return
	}

	defer func() {
		switch err {
		case nil:
			tx.Commit()
		default:
			tx.Rollback()
		}
	}()

	valuesAux := make([]interface{}, 0, len(values))

	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	result, err := tx.Exec(sql, valuesAux...)
	if err != nil {
		tx.Rollback()
		log.Printf("sql = %+v\n", sql)
		err = fmt.Errorf("could not peform sql: %v", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		// err here is nil, ever!
		err = fmt.Errorf("could not rows affected: %v", err)
		return
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	resultByte, err = json.Marshal(data)

	return
}

// ExecuteScripts run sql templates created by users
func ExecuteScripts(method, sql string, values []interface{}) (result []byte, err error) {
	switch method {
	case "GET":
		result, err = Query(sql, values...)
	case "POST", "PUT", "PATCH", "DELETE":
		result, err = WriteSQL(sql, values)
	default:
		err = fmt.Errorf("invalid method %s", err)
	}

	return
}
