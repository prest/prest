package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/caarlos0/env"
	"github.com/jmoiron/sqlx"
	// Used pg drive on sqlx
	_ "github.com/lib/pq"

	"database/sql"

	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/config"
)

const (
	pageNumberKey   = "_page"
	pageSizeKey     = "_page_size"
	defaultPageSize = 10
)

// Conn connect on PostgreSQL
// Used sqlx
func Conn() (db *sqlx.DB) {
	cfg := config.Prest{}
	env.Parse(&cfg)

	db, err := sqlx.Connect("postgres", fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=disable", cfg.PGUser, cfg.PGDatabase, cfg.PGHost, cfg.PGPort))
	if err != nil {
		panic(fmt.Sprintf("Unable to connection to database: %v\n", err))
	}
	return
}

// chkInvaidIdentifier return true if identifier is invalid
func chkInvaidIdentifier(identifer string) bool {
	if len(identifer) > 63 ||
		unicode.IsDigit([]rune(identifer)[0]) {
		return true
	}

	for _, v := range identifer {
		if !unicode.IsLetter(v) &&
			!unicode.IsDigit(v) &&
			v != '_' &&
			v != '.' {
			return true
		}
	}
	return false
}

// WhereByRequest create interface for queries + where
func WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error) {
	//whereMap := make(map[string]string)
	whereKey := []string{}
	whereValues := []string{}

	u, _ := url.Parse(r.URL.String())
	pid := initialPlaceholderID
	for key, val := range u.Query() {
		if !strings.HasPrefix(key, "_") {
			keyInfo := strings.Split(key, ":")
			if len(keyInfo) > 1 {
				switch keyInfo[1] {
				case "jsonb":
					jsonField := strings.Split(keyInfo[0], "->>")
					if chkInvaidIdentifier(jsonField[0]) ||
						chkInvaidIdentifier(jsonField[1]) {
						err = errors.New("Invalid identifier")
						return
					}
					whereKey = append(whereKey, fmt.Sprintf("%s->>'%s'=$%d", jsonField[0], jsonField[1], pid))
					whereValues = append(whereValues, val[0])
				default:
					if chkInvaidIdentifier(keyInfo[0]) {
						err = errors.New("Invalid identifier")
						return
					}
					whereKey = append(whereKey, fmt.Sprintf("%s=$%d", keyInfo[0], pid))
					whereValues = append(whereValues, val[0])
				}
				continue
			}
			if chkInvaidIdentifier(key) {
				err = errors.New("Invalid identifier")
				return
			}

			whereKey = append(whereKey, fmt.Sprintf("%s=$%d", key, pid))
			whereValues = append(whereValues, val[0])

			pid++
		}
	}

	for i := 0; i < len(whereKey); i++ {
		if whereSyntax == "" {
			whereSyntax += whereKey[i]
		} else {
			whereSyntax += " AND " + whereKey[i]
		}

		values = append(values, whereValues[i])
	}

	return
}

// Query process queries
func Query(SQL string, params ...interface{}) (jsonData []byte, err error) {
	db := Conn()

	prepare, err := db.Prepare(SQL)

	if err != nil {
		return
	}

	rows, err := prepare.Query(params...)
	if err != nil {
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return
	}

	count := len(columns)
	tableData := make([]map[string]interface{}, 0)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)
	for rows.Next() {
		for i := 0; i < count; i++ {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)
		entry := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			entry[col] = v
		}
		tableData = append(tableData, entry)
	}
	jsonData, err = json.Marshal(tableData)

	return
}

// PaginateIfPossible func
func PaginateIfPossible(r *http.Request) (paginatedQuery string, err error) {
	u, _ := url.Parse(r.URL.String())
	values := u.Query()
	if _, ok := values[pageNumberKey]; !ok {
		paginatedQuery = ""
		return
	}
	pageNumber, err := strconv.Atoi(values[pageNumberKey][0])
	if err != nil {
		return
	}
	pageSize := defaultPageSize
	if size, ok := values[pageSizeKey]; ok {
		pageSize, err = strconv.Atoi(size[0])
		if err != nil {
			return
		}
	}
	paginatedQuery = fmt.Sprintf("LIMIT %d OFFSET(%d - 1) * %d", pageSize, pageNumber, pageSize)
	return
}

// Insert execute insert sql into a table
func Insert(database, schema, table string, body api.Request) (jsonData []byte, err error) {
	var result sql.Result
	var rowsAffected int64

	fields := make([]string, 0)
	values := make([]string, 0)
	for key, value := range body.Data {
		fields = append(fields, key)
		values = append(values, value)
	}
	colsName := strings.Join(fields, ", ")
	colsValue := strings.Join(values, "', '")
	sql := fmt.Sprintf("INSERT INTO %s.%s.%s (%s) VALUES ('%s')", database, schema, table, colsName, colsValue)

	db := Conn()
	result, err = db.Exec(sql)
	if err != nil {
		return
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	jsonData, err = json.Marshal(data)
	return
}

// Delete execute delete sql into a table
func Delete(database, schema, table, where string, whereValues []interface{}) (jsonData []byte, err error) {
	var result sql.Result
	var rowsAffected int64

	sql := fmt.Sprintf("DELETE FROM %s.%s.%s", database, schema, table)
	if where != "" {
		sql = fmt.Sprint(
			sql,
			" WHERE ",
			where)
	}

	db := Conn()
	result, err = db.Exec(sql, whereValues...)
	if err != nil {
		return
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	jsonData, err = json.Marshal(data)
	return
}

// Update execute update sql into a table
func Update(database, schema, table, where string, whereValues []interface{}, body api.Request) (jsonData []byte, err error) {
	var result sql.Result
	var rowsAffected int64

	fields := []string{}
	values := make([]interface{}, 0)
	pid := len(whereValues) + 1 // placeholder id
	for key, value := range body.Data {
		fields = append(fields, fmt.Sprintf("%s=$%d", key, pid))
		values = append(values, value)
		pid++
	}
	setSyntax := strings.Join(fields, ", ")

	sql := fmt.Sprintf("UPDATE %s.%s.%s SET %s", database, schema, table, setSyntax)

	if where != "" {
		sql = fmt.Sprint(
			sql,
			" WHERE ",
			where)
		values = append(values, whereValues...)
	}

	db := Conn()
	//result, err = db.Exec(sql, values)
	stmt, err := db.Prepare(sql)
	if err != nil {
		return
	}

	valuesAux := make([]interface{}, 0, len(values))

	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	result, err = stmt.Exec(valuesAux...)
	if err != nil {
		return
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return
	}

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	jsonData, err = json.Marshal(data)
	return
}
