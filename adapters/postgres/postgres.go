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
	if len(identifer) > 63 || unicode.IsDigit([]rune(identifer)[0]) {
		return true
	}

	for _, v := range identifer {
		if !unicode.IsLetter(v) && !unicode.IsDigit(v) && v != '_' && v != '.' && v != '-' {
			return true
		}
	}
	return false
}

// WhereByRequest create interface for queries + where
func WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error) {
	whereKey := []string{}
	whereValues := []string{}
	var value, op string

	op, err = GetQueryOperator("$eq")
	if err != nil {
		// Never throw an error, amen
		return
	}

	pid := initialPlaceholderID
	for key, val := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {

			value = val[0]
			if val[0] != "" {
				opValues := strings.Split(val[0], ".")
				correctLength := len(opValues) == 2
				hasPreffix := strings.HasPrefix(opValues[0], "$")
				if hasPreffix {
					op = opValues[0]
					if correctLength {
						value = opValues[1]
					} else {
						value = ""
					}
					op, err = GetQueryOperator(op)
					if err != nil {
						return
					}
				}
			}

			keyInfo := strings.Split(key, ":")

			if len(keyInfo) > 1 {
				switch keyInfo[1] {
				case "jsonb":
					jsonField := strings.Split(keyInfo[0], "->>")
					if chkInvalidIdentifier(jsonField[0]) || chkInvalidIdentifier(jsonField[1]) {
						err = fmt.Errorf("invalid identifier: %+v", jsonField)
						return
					}

					whereKey = append(whereKey, fmt.Sprintf("%s->>'%s' %s $%d", jsonField[0], jsonField[1], op, pid))
					whereValues = append(whereValues, value)
				default:
					if chkInvalidIdentifier(keyInfo[0]) {
						err = fmt.Errorf("invalid identifier: %s", keyInfo[0])
						return
					}
				}
				pid++
				continue
			}

			if chkInvalidIdentifier(key) {
				err = fmt.Errorf("invalid identifier: %s", key)
				return
			}

			if value != "" {
				whereKey = append(whereKey, fmt.Sprintf("%s %s $%d", key, op, pid))
				whereValues = append(whereValues, value)

				pid++
			} else {
				whereKey = append(whereKey, fmt.Sprintf("%s %s", key, op))
			}
		}
	}

	for i := 0; i < len(whereKey); i++ {
		if whereSyntax == "" {
			whereSyntax += whereKey[i]
		} else {
			whereSyntax += " AND " + whereKey[i]
		}

		if i < len(whereValues) {
			values = append(values, whereValues[i])
		}
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
	queries := r.URL.Query()

	if queries.Get("_join") == "" {
		return
	}

	joinArgs := strings.Split(queries.Get("_join"), ":")
	chk := chkInvalidIdentifier

	if len(joinArgs) != 5 {
		err = errors.New("Invalid number of arguments in join statement")
		return
	}

	if chk(joinArgs[1]) || chk(joinArgs[2]) || chk(joinArgs[4]) {
		err = errors.New("Invalid identifier")
		return nil, err
	}

	op, err := GetQueryOperator(joinArgs[3])
	if err != nil {
		return nil, err
	}

	joinQuery := fmt.Sprintf(" %s JOIN %s ON %s %s %s ", strings.ToUpper(joinArgs[0]), joinArgs[1], joinArgs[2], op, joinArgs[4])
	values = append(values, joinQuery)

	return
}

// SelectFields query
func SelectFields(fields []string) (sql string, err error) {
	if len(fields) == 0 {
		err = errors.New("you must select at least one field")
		return
	}

	for _, field := range fields {
		if field != "*" && chkInvalidIdentifier(field) {
			err = fmt.Errorf("invalid identifier %s", field)
			return
		}
	}

	sql = fmt.Sprintf("SELECT %s FROM", strings.Join(fields, ","))
	return
}

// OrderByRequest implements ORDER BY in queries
func OrderByRequest(r *http.Request) (values string, err error) {
	queries := r.URL.Query()
	reqOrder := queries.Get("_order")

	if reqOrder != "" {
		values = " ORDER BY "
		orderingArr := strings.Split(reqOrder, ",")

		for i, field := range orderingArr {
			if chkInvalidIdentifier(field) {
				err = errors.New("Invalid identifier")
				values = ""
				return
			}

			if strings.HasPrefix(field, "-") {
				field = fmt.Sprintf("%s DESC", field[1:])
			}

			values = fmt.Sprintf("%s %s", values, field)

			// if have next order, append a comma
			if i < len(orderingArr)-1 {
				values = fmt.Sprintf("%s ,", values)
			}
		}
	}
	return
}

// CountByRequest implements COUNT(fields) OPERTATION
func CountByRequest(req *http.Request) (countQuery string, err error) {
	queries := req.URL.Query()
	countFields := queries.Get("_count")

	if countFields == "" {
		return
	}

	for _, field := range strings.Split(countFields, ",") {
		if field != "*" && chkInvalidIdentifier(field) {
			err = errors.New("Invalid identifier")
			return
		}
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
	if chkInvalidIdentifier(database) || chkInvalidIdentifier(schema) || chkInvalidIdentifier(table) {
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
	db := connection.MustGet()
	tx, err := db.Begin()
	if err != nil {
		log.Printf("could not begin transaction: %v\n", err)
		return
	}
	stmtPK, err := tx.Prepare(statements.SelectPKTableName)
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", statements.SelectPKTableName, err)
		return
	}
	var pkName string
	pkRow, err := stmtPK.Query(table)
	if err != nil {
		return
	}
	for pkRow.Next() {
		err = pkRow.Scan(&pkName)
		if err != nil {
			return
		}
	}
	err = pkRow.Close()
	if err != nil {
		return
	}

	sql := fmt.Sprintf("INSERT INTO %s.%s.%s (%s) VALUES (%s)", database, schema, table, colsName, colPlaceholder)
	if pkName != "" {
		sql = fmt.Sprintf("INSERT INTO %s.%s.%s (%s) VALUES (%s) RETURNING %s", database, schema, table, colsName, colPlaceholder, pkName)
	}

	defer func() {
		switch err {
		case nil:
			tx.Commit()
		default:
			tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(sql)
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", sql, err)
		return
	}

	valuesAux := make([]interface{}, 0, len(values))

	for i := 0; i < len(values); i++ {
		valuesAux = append(valuesAux, values[i])
	}

	var lastID interface{}
	if pkName != "" {
		result := stmt.QueryRow(valuesAux...)
		err = result.Scan(&lastID)
		if err != nil {
			return
		}
	} else {
		_, err = stmt.Exec(valuesAux...)
		if err != nil {
			return
		}
	}

	data := make(map[string]interface{})
	for i := range fields {
		data[fields[i]] = values[i]
	}
	if pkName != "" {
		data[pkName] = lastID
	}
	jsonData, err = json.Marshal(data)
	return
}

// Delete execute delete sql into a table
func Delete(database, schema, table, where string, whereValues []interface{}) (jsonData []byte, err error) {
	var result sql.Result
	var rowsAffected int64

	if chkInvalidIdentifier(database) || chkInvalidIdentifier(schema) || chkInvalidIdentifier(table) {
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

	defer func() {
		switch err {
		case nil:
			tx.Commit()
		default:
			tx.Rollback()
		}
	}()

	result, err = tx.Exec(sql, whereValues...)
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
	if chkInvalidIdentifier(database) || chkInvalidIdentifier(schema) || chkInvalidIdentifier(table) {
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

	defer func() {
		switch err {
		case nil:
			tx.Commit()
		default:
			tx.Rollback()
		}
	}()

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
	case "ne":
		return "!=", nil
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
	case "notnull":
		return "IS NOT NULL", nil
	case "null":
		return "IS NULL", nil
	}

	err := errors.New("Invalid operator")
	return "", err

}

// TablePermissions get tables permissions based in prest configuration
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

// FieldsPermissions get fields permissions based in prest configuration
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
