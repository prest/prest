package postgres

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
	defaultPageSize = "10"
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

// WhereByRequest create interface for queries + where
func WhereByRequest(r *http.Request) (whereSyntax map[string]string) {
	whereSyntax = make(map[string]string)
	u, _ := url.Parse(r.URL.String())
	for key, val := range u.Query() {
		if !strings.HasPrefix(key, "_") {
			keyInfo := strings.Split(key, ":")
			if len(keyInfo) > 1 {
				switch keyInfo[1] {
				case "jsonb":
					jsonField := strings.Split(keyInfo[0], "->>")
					whereSyntax[fmt.Sprintf("%s->>'%s'=?", jsonField[0], jsonField[1])] = val[0]
				default:
					whereSyntax[fmt.Sprintf("%s=?", keyInfo[0])] = val[0]
				}
				continue
			}
			whereSyntax[fmt.Sprintf("%s=?", key)] = val[0]
		}
	}

	return
}

// Query process queries
func Query(SQL string, params ...interface{}) (jsonData []byte, err error) {
	db := Conn()
	rows, err := db.Queryx(SQL, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
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
func PaginateIfPossible(r *http.Request) (paginatedQuery string) {
	u, _ := url.Parse(r.URL.String())
	values := u.Query()
	if _, ok := values[pageNumberKey]; !ok {
		paginatedQuery = ""
		return
	}
	pageNumber := values[pageNumberKey][0]
	pageSize := defaultPageSize
	if size, ok := values[pageSizeKey]; ok {
		pageSize = size[0]
	}
	paginatedQuery = fmt.Sprintf("LIMIT %s OFFSET(%s - 1) * %s", pageSize, pageNumber, pageSize)
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
func Delete(database, schema, table, where string) (jsonData []byte, err error) {
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

// Update execute update sql into a table
func Update(database, schema, table, where string, body api.Request) (jsonData []byte, err error) {
	var result sql.Result
	var rowsAffected int64

	fields := []string{}
	for key, value := range body.Data {
		fields = append(fields, fmt.Sprintf("%s='%s'", key, value))
	}
	setSyntax := strings.Join(fields, ", ")

	sql := fmt.Sprintf("UPDATE %s.%s.%s SET %s", database, schema, table, setSyntax)

	if where != "" {
		sql = fmt.Sprint(
			sql,
			" WHERE ",
			where)
	}

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
