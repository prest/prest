package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"database/sql"

	"github.com/nuveo/prest/adapters/postgres/connection"
	"github.com/nuveo/prest/api"
	"github.com/nuveo/prest/config"
	"github.com/nuveo/prest/statements"
)

const (
	pageNumberKey   = "_page"
	pageSizeKey     = "_page_size"
	defaultPageSize = 10
)

// chkInvalidIdentifier return true if identifier is invalid
func chkInvalidIdentifier(identifer string) bool {
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

	pid := initialPlaceholderID
	for key, val := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {
			keyInfo := strings.Split(key, ":")
			if len(keyInfo) > 1 {
				switch keyInfo[1] {
				case "jsonb":
					jsonField := strings.Split(keyInfo[0], "->>")
					if chkInvalidIdentifier(jsonField[0]) ||
						chkInvalidIdentifier(jsonField[1]) {
						err = errors.New("Invalid identifier")
						return
					}
					whereKey = append(whereKey, fmt.Sprintf("%s->>'%s'=$%d", jsonField[0], jsonField[1], pid))
					whereValues = append(whereValues, val[0])
				default:
					if chkInvalidIdentifier(keyInfo[0]) {
						err = errors.New("Invalid identifier")
						return
					}
				}
				continue
			}
			if chkInvalidIdentifier(key) {
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

// DatabaseClause return a SELECT `query`
func DatabaseClause(req *http.Request) (query string) {
	queries := req.URL.Query()
	hasCount := queries.Get("_count")

	if hasCount != "" {
		query = fmt.Sprintf(statements.DatabasesSelect, statements.FieldCountDatabaseName)
	} else {
		query = fmt.Sprintf(statements.DatabasesSelect, statements.FieldDatabaseName)
	}
	return
}

// SchemaClause return a SELECT `query`
func SchemaClause(req *http.Request) (query string) {
	queries := req.URL.Query()
	hasCount := queries.Get("_count")

	if hasCount != "" {
		query = fmt.Sprintf(statements.SchemasSelect, statements.FieldCountSchemaName)
	} else {
		query = fmt.Sprintf(statements.SchemasSelect, statements.FieldSchemaName)
	}
	return
}

// JoinByRequest implements join in queries
func JoinByRequest(r *http.Request) (values []string, err error) {
	joinValues := []string{}
	joinStatements := r.URL.Query()["_join"]

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

func SelectFields(fields []string) (string, error) {
	if len(fields) == 0 {
		return "", errors.New("You must select at least one field.")
	}
	return fmt.Sprintf("SELECT %s FROM", strings.Join(fields, ",")), nil
}

// OrderByRequest implements ORDER BY in queries
func OrderByRequest(r *http.Request) (string, error) {
	var values string
	reqOrder := r.URL.Query()["_order"]

	if len(reqOrder) > 0 {
		values = " ORDER BY "

		// get last order in request url
		ordering := reqOrder[len(reqOrder)-1]
		orderingArr := strings.Split(ordering, ",")

		for i, s := range orderingArr {
			field := s

			if strings.HasPrefix(s, "-") {
				field = fmt.Sprintf("%s DESC", s[1:])
			}

			values = fmt.Sprintf("%s %s", values, field)

			// if have next order, append a comma
			if i < len(orderingArr)-1 {
				values = fmt.Sprintf("%s ,", values)
			}
		}
	}
	return values, nil
}

// CountByRequest implements COUNT(fields) OPERTATION
func CountByRequest(req *http.Request) (countQuery string) {
	queries := req.URL.Query()
	countFields := queries.Get("_count")

	if countFields == "" {
		return
	}
	countQuery = fmt.Sprintf("SELECT COUNT(%s) FROM", countFields)

	return
}

// Query process queries
func Query(SQL string, params ...interface{}) (jsonData []byte, err error) {
	db := connection.MustGet()
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

// QueryCount process queries with count
func QueryCount(SQL string, params ...interface{}) ([]byte, error) {
	validQuery := chkInvalidIdentifier(SQL)
	if !validQuery {
		return nil, errors.New("Invalid characters in the query")
	}

	db := connection.MustGet()
	prepare, err := db.Prepare(SQL)
	if err != nil {
		return nil, err
	}

	var result struct {
		Count int64 `json:"count"`
	}

	row := prepare.QueryRow(params...)
	if err := row.Scan(&result.Count); err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// PaginateIfPossible func
func PaginateIfPossible(r *http.Request) (paginatedQuery string, err error) {
	values := r.URL.Query()
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
	allowed := TablePermissions(table, "write")
	if !allowed {
		return nil, errors.New("Insuficient table permissions")
	}

	if chkInvalidIdentifier(database) ||
		chkInvalidIdentifier(schema) ||
		chkInvalidIdentifier(table) {
		err = errors.New("Insert: Invalid identifier")
		return
	}

	fields := make([]string, 0)
	values := make([]interface{}, 0)
	for key, value := range body.Data {
		if chkInvalidIdentifier(key) {
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

	sql := fmt.Sprintf("INSERT INTO %s.%s.%s (%s) VALUES (%s) RETURNING id;", database, schema, table, colsName, colPlaceholder)

	db := connection.MustGet()
	tx, err := db.Begin()
	if err != nil {
		log.Printf("could not begin transaction: %v\n", err)
		return
	}

	stmt, err := tx.Prepare(sql)
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", sql, err)
		return
	}

	valuesAux := make([]interface{}, 0, len(values))

	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	var lastID int
	result := stmt.QueryRow(valuesAux...)
	err = result.Scan(&lastID)
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			log.Printf("could not commit: %v\n", err)
		}
	}()

	data := make(map[string]interface{})
	for i := range fields {
		data[fields[i]] = values[i]
	}
	data["id"] = lastID
	jsonData, err = json.Marshal(data)
	return
}

// Delete execute delete sql into a table
func Delete(database, schema, table, where string, whereValues []interface{}) (jsonData []byte, err error) {
	allowed := TablePermissions(table, "delete")
	if !allowed {
		return nil, errors.New("Insuficient table permissions")
	}

	var result sql.Result
	var rowsAffected int64

	if chkInvalidIdentifier(database) ||
		chkInvalidIdentifier(schema) ||
		chkInvalidIdentifier(table) {
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

	db := connection.MustGet()
	tx, err := db.Begin()
	if err != nil {
		log.Printf("could not begin transaction: %v\n", err)
		return
	}

	result, err = tx.Exec(sql, whereValues...)
	if err != nil {
		return
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return
	}

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}

		err = tx.Commit()
		if err != nil {
			log.Printf("could not commit: %v\n", err)
		}
	}()

	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	jsonData, err = json.Marshal(data)
	return
}

// Update execute update sql into a table
func Update(database, schema, table, where string, whereValues []interface{}, body api.Request) (jsonData []byte, err error) {
	allowed := TablePermissions(table, "write")
	if !allowed {
		return nil, errors.New("Insuficient table permissions")
	}

	if chkInvalidIdentifier(database) ||
		chkInvalidIdentifier(schema) ||
		chkInvalidIdentifier(table) {
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

	db := connection.MustGet()
	tx, err := db.Begin()
	if err != nil {
		log.Printf("could not begin transaction: %v\n", err)
		return
	}

	stmt, err := tx.Prepare(sql)
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", sql, err)
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

	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
		if err != nil {
			log.Printf("could not commit: %v\n", err)
		}
	}()

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

// get tables permissions based in prest configuration
func TablePermissions(table string, op string) bool {
	restrict := config.PREST_CONF.AccessConf.Restrict
	if !restrict {
		return true
	}

	tables := config.PREST_CONF.AccessConf.Tables
	for _, t := range tables {
		if t.Name == table {
			for _, p := range t.Permissions {
				if p == op {
					return true
				}
			}
		}
	}
	return false
}

// get fields permissions based in prest configuration
func FieldsPermissions(table string, cols []string, op string) []string {
	restrict := config.PREST_CONF.AccessConf.Restrict
	if !restrict {
		return cols
	}

	var permittedCols []string
	tables := config.PREST_CONF.AccessConf.Tables
	for _, t := range tables {
		if t.Name == table {
			for _, f := range t.Fields {
				for _, col := range cols {
					// return all permitted fields if have "*" in SELECT
					if op == "read" && col == "*" {
						return t.Fields
					}

					if col == f {
						permittedCols = append(permittedCols, col)
					}
				}
			}
			return permittedCols
		}
	}
	return nil
}

// ColumnsByRequest extract columns and return as array of strings
func ColumnsByRequest(r *http.Request) []string {
	u, _ := r.URL.Parse(r.URL.String())
	columnsArr := u.Query()["_select"]
	var columns []string

	for _, j := range columnsArr {
		cArgs := strings.Split(j, ",")
		for _, columnName := range cArgs {
			if len(columnName) > 0 {
				columns = append(columns, columnName)
			}
		}
	}
	if len(columns) == 0 {
		return []string{"*"}
	}
	return columns
}
