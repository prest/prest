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

var db *sqlx.DB

func init() {
	cfg := config.Prest{}
	config.Parse(&cfg)
	var err error
	dbURI := fmt.Sprintf("user=%s dbname=%s host=%s port=%v sslmode=disable", cfg.PGUser, cfg.PGDatabase, cfg.PGHost, cfg.PGPort)
	if cfg.PGPass != "" {
		dbURI += " password=" + cfg.PGPass
	}
	db, err = sqlx.Connect("postgres", dbURI)
	if err != nil {
		panic(fmt.Sprintf("Unable to connection to database: %v\n", err))
	}
	db.SetMaxIdleConns(cfg.PGMaxIdleConn)
	db.SetMaxOpenConns(cfg.PGMAxOpenConn)
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

// JoinByRequest implements join in queries
func JoinByRequest(r *http.Request) (values []string, err error) {
	joinValues := []string{}

	u, _ := url.Parse(r.URL.String())
	joinStatements := u.Query()["_join"]

	for _, j := range joinStatements {
		joinArgs := strings.Split(j, ":")

		if len(joinArgs) != 5 {
			err = errors.New("Invalid number of arguments in join statement")
			return nil, err
		}

		op, err := GetQueryOperator(joinArgs[3])
		if err != nil {
			return nil, err
		}

		joinQuery := fmt.Sprintf(" %s JOIN %s ON %s %s %s ", strings.ToUpper(joinArgs[0]), joinArgs[1], joinArgs[2], op, joinArgs[4])
		joinValues = append(joinValues, joinQuery)
	}

	return joinValues, nil
}

// Query process queries
func Query(SQL string, params ...interface{}) (jsonData []byte, err error) {
	validQuery := chkInvaidIdentifier(SQL)
	if !validQuery {
		err := errors.New("Invalid characters in the query")
		return nil, err
	}

	// db := connection.Conn()

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

	if chkInvaidIdentifier(database) ||
		chkInvaidIdentifier(schema) ||
		chkInvaidIdentifier(table) {
		err = errors.New("Insert: Invalid identifier")
		return
	}

	var result sql.Result
	var rowsAffected int64

	fields := make([]string, 0)
	values := make([]interface{}, 0)
	for key, value := range body.Data {
		if chkInvaidIdentifier(key) {
			err = errors.New("Insert: Invalid identifier")
			return
		}
		fields = append(fields, key)
		values = append(values, value)
	}

	colsName := strings.Join(fields, ", ")
	colPlaceholder := ""
	for i := 1; i < len(values)+1; i++ {
		if colPlaceholder != "" {
			colPlaceholder += ","
		}
		colPlaceholder += fmt.Sprintf("$%d", i)
	}

	sql := fmt.Sprintf("INSERT INTO %s.%s.%s (%s) VALUES (%s)", database, schema, table, colsName, colPlaceholder)

	// db := connection.Conn()
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

// Delete execute delete sql into a table
func Delete(database, schema, table, where string, whereValues []interface{}) (jsonData []byte, err error) {
	var result sql.Result
	var rowsAffected int64

	if chkInvaidIdentifier(database) ||
		chkInvaidIdentifier(schema) ||
		chkInvaidIdentifier(table) {
		err = errors.New("Delete: Invalid identifier")
		return
	}

	sql := fmt.Sprintf("DELETE FROM %s.%s.%s", database, schema, table)
	if where != "" {
		sql = fmt.Sprint(
			sql,
			" WHERE ",
			where)
	}

	// db := connection.Conn()
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

	if chkInvaidIdentifier(database) ||
		chkInvaidIdentifier(schema) ||
		chkInvaidIdentifier(table) {
		err = errors.New("Update: Invalid identifier")
		return
	}

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
		values = append(whereValues, values...)
	}

	// db := connection.Conn()
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

// GetQueryOperator identify operator on a join
func GetQueryOperator(op string) (string, error) {
	op = strings.Replace(op, "$", "", -1)
	op = strings.Replace(op, " ", "", -1)

	switch op {
	case "eq":
		return "=", nil
	case "gt":
		return ">", nil
	case "gte":
		return ">=", nil
	case "lt":
		return "<", nil
	case "lte":
		return "<=", nil
	case "in":
		return "IN", nil
	case "nin":
		return "NOT IN", nil
	}

	err := errors.New("Invalid operator")
	return "", err

}
