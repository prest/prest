package postgres

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/prest/adapters/postgres/connection"
	"github.com/prest/adapters/postgres/internal/scanner"
	"github.com/prest/config"
	"github.com/prest/statements"
)

const (
	pageNumberKey   = "_page"
	pageSizeKey     = "_page_size"
	defaultPageSize = 10
)

var removeOperatorRegex *regexp.Regexp
var insertTableNameRegex *regexp.Regexp
var groupRegex *regexp.Regexp

// ErrBodyEmpty err throw when body is empty
var ErrBodyEmpty = errors.New("body is empty")

func init() {
	removeOperatorRegex = regexp.MustCompile(`\$[a-z]+.`)
	insertTableNameRegex = regexp.MustCompile(`(?i)INTO\s+([\w|\.]*\.)*(\w+)\s*\(`)
	groupRegex = regexp.MustCompile(`\(([^\)]+)\)`)
}

// chkInvalidIdentifier return true if identifier is invalid
func chkInvalidIdentifier(identifer ...string) bool {
	for _, ival := range identifer {
		if ival == "" || len(ival) > 63 || unicode.IsDigit([]rune(ival)[0]) {
			return true
		}

		for _, v := range ival {
			if !unicode.IsLetter(v) &&
				!unicode.IsDigit(v) &&
				v != '(' &&
				v != ')' &&
				v != '_' &&
				v != '.' &&
				v != '-' &&
				v != '*' &&
				v != '[' &&
				v != ']' {
				return true
			}
		}
	}
	return false
}

// WhereByRequest create interface for queries + where
func WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error) {
	whereKey := []string{}
	whereValues := []string{}
	var value, op string

	pid := initialPlaceholderID
	for key, val := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {
			value = val[0]
			if val[0] != "" {
				op = removeOperatorRegex.FindString(val[0])
				op = strings.Replace(op, ".", "", -1)
				if op == "" {
					op = "$eq"
				}
				value = removeOperatorRegex.ReplaceAllString(val[0], "")
				op, err = GetQueryOperator(op)
				if err != nil {
					return
				}
			}

			keyInfo := strings.Split(key, ":")

			if len(keyInfo) > 1 {
				switch keyInfo[1] {
				case "jsonb":
					jsonField := strings.Split(keyInfo[0], "->>")
					if chkInvalidIdentifier(jsonField[0], jsonField[1]) {
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

// SetByRequest create a set clause for SQL
func SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error) {
	body := make(map[string]interface{})
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		err = ErrBodyEmpty
		return
	}

	fields := make([]string, 0)
	for key, value := range body {
		if chkInvalidIdentifier(key) {
			err = errors.New("Set: Invalid identifier")
			return
		}
		fields = append(fields, fmt.Sprintf("%s=$%d", key, initialPlaceholderID))

		switch value.(type) {
		case []interface{}:
			values = append(values, parseArray(value))
		default:
			values = append(values, value)
		}

		initialPlaceholderID++
	}
	setSyntax = strings.Join(fields, ", ")
	return
}

// ParseInsertRequest create insert SQL
func ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error) {
	body := make(map[string]interface{})
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		err = ErrBodyEmpty
		return
	}

	fields := make([]string, 0)
	for key, value := range body {
		if chkInvalidIdentifier(key) {
			err = errors.New("Insert: Invalid identifier")
			return
		}
		fields = append(fields, key)

		switch value.(type) {
		case []interface{}:
			values = append(values, parseArray(value))
		default:
			values = append(values, value)
		}
	}

	colsName = strings.Join(fields, ", ")
	for i := 1; i < len(values)+1; i++ {
		if colsValue != "" {
			colsValue += ","
		}
		colsValue += fmt.Sprintf("$%d", i)
	}
	return
}

// DatabaseClause return a SELECT `query`
func DatabaseClause(req *http.Request) (query string, hasCount bool) {
	queries := req.URL.Query()
	countQuery := queries.Get("_count")

	if countQuery != "" {
		hasCount = true
		query = fmt.Sprintf(statements.DatabasesSelect, statements.FieldCountDatabaseName)
	} else {
		query = fmt.Sprintf(statements.DatabasesSelect, statements.FieldDatabaseName)
	}
	return
}

// SchemaClause return a SELECT `query`
func SchemaClause(req *http.Request) (query string, hasCount bool) {
	queries := req.URL.Query()
	countQuery := queries.Get("_count")

	if countQuery != "" {
		hasCount = true
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

	if len(joinArgs) != 5 {
		err = errors.New("Invalid number of arguments in join statement")
		return
	}

	if chkInvalidIdentifier(joinArgs[1], joinArgs[2], joinArgs[4]) {
		err = errors.New("Invalid identifier")
		return
	}

	op, err := GetQueryOperator(joinArgs[3])
	if err != nil {
		return
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
		if chkInvalidIdentifier(field) {
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
func Query(SQL string, params ...interface{}) (sc Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	SQL = fmt.Sprintf("SELECT json_agg(s) FROM (%s) s", SQL)
	// Debug mode
	if config.PrestConf.Debug {
		log.Println(SQL)
	}
	prepare, err := db.Prepare(SQL)
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	defer prepare.Close()
	var jsonData []byte
	err = prepare.QueryRow(params...).Scan(&jsonData)
	if len(jsonData) == 0 {
		jsonData = []byte("[]")
	}
	sc = &scanner.PrestScanner{
		Error:   err,
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
	return
}

// QueryCount process queries with count
func QueryCount(SQL string, params ...interface{}) (sc Scanner) {
	db, err := connection.Get()
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	// Debug mode
	if config.PrestConf.Debug {
		log.Println(SQL)
	}
	prepare, err := db.Prepare(SQL)
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	var result struct {
		Count int64 `json:"count"`
	}

	row := prepare.QueryRow(params...)
	if err = row.Scan(&result.Count); err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	var byt []byte
	byt, err = json.Marshal(result)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(byt),
	}
	return
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

func parseArray(value interface{}) string {
	switch value.(type) {
	case []interface{}:
		var aux string
		for _, v := range value.([]interface{}) {
			if aux != "" {
				aux += ","
			}
			aux += parseArray(v)
		}
		return "{" + aux + "}"
	case string:
		aux := value.(string)
		aux = strings.Replace(aux, `\`, `\\`, -1)
		aux = strings.Replace(aux, `"`, `\"`, -1)
		return `"` + aux + `"`
	case int:
		return strconv.Itoa(value.(int))
	}
	return ""
}

// Insert execute insert sql into a table
func Insert(SQL string, params ...interface{}) (sc Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	tx, err := db.Begin()
	if err != nil {
		log.Printf("could not begin transaction: %v\n", err)
		sc = &scanner.PrestScanner{Error: err}
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
	tableName := insertTableNameRegex.FindStringSubmatch(SQL)
	if len(tableName) < 2 {
		err = errors.New("unable to find table name")
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	// Debug mode
	if config.PrestConf.Debug {
		log.Println(SQL)
	}
	SQL = fmt.Sprintf("%s RETURNING row_to_json(%s)", SQL, tableName[2])
	stmt, err := tx.Prepare(SQL)
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", SQL, err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	var jsonData []byte
	err = stmt.QueryRow(params...).Scan(&jsonData)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
	return
}

// Delete execute delete sql into a table
func Delete(SQL string, params ...interface{}) (sc Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	tx, err := db.Begin()
	if err != nil {
		log.Printf("could not begin transaction: %v\n", err)
		sc = &scanner.PrestScanner{Error: err}
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
	// Debug mode
	if config.PrestConf.Debug {
		log.Println(SQL)
	}
	var result sql.Result
	var rowsAffected int64
	result, err = tx.Exec(SQL, params...)
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var jsonData []byte
	jsonData, err = json.Marshal(data)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
	return
}

// Update execute update sql into a table
func Update(SQL string, params ...interface{}) (sc Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	tx, err := db.Begin()
	if err != nil {
		log.Printf("could not begin transaction: %v\n", err)
		sc = &scanner.PrestScanner{Error: err}
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
	stmt, err := tx.Prepare(SQL)
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", SQL, err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	// Debug mode
	if config.PrestConf.Debug {
		log.Println(SQL)
	}
	var result sql.Result
	var rowsAffected int64
	result, err = stmt.Exec(params...)
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var jsonData []byte
	jsonData, err = json.Marshal(data)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
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
	restrict := config.PrestConf.AccessConf.Restrict
	if !restrict {
		return true
	}

	tables := config.PrestConf.AccessConf.Tables
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
func FieldsPermissions(r *http.Request, table string, op string) (fields []string, err error) {
	restrict := config.PrestConf.AccessConf.Restrict
	cols := ColumnsByRequest(r)
	queries := r.URL.Query()
	if queries.Get("_groupby") != "" {
		cols, err = normalizeAll(cols)
		if err != nil {
			return
		}
	}
	if !restrict {
		fields = cols
		return
	}

	tables := config.PrestConf.AccessConf.Tables
	for _, t := range tables {
		if t.Name == table {
			for _, col := range cols {
				// return all permitted fields if have "*" in SELECT
				if op == "read" && col == "*" {
					fields = t.Fields
					return
				}
				pField := checkField(col, t.Fields)
				if pField != "" {
					fields = append(fields, pField)
				}
			}
			return
		}
	}
	return nil, errors.New("0 tables configured")
}

func checkField(col string, fields []string) (p string) {
	// regex get field from func group
	fieldName := groupRegex.FindStringSubmatch(col)
	for _, f := range fields {
		if len(fieldName) == 2 && fieldName[1] == f {
			p = col
			return
		}
		if col == f {
			p = col
			return
		}
	}
	return
}

func normalizeAll(cols []string) (pCols []string, err error) {
	for _, col := range cols {
		var gf string
		gf, err = normalizeColumn(col)
		if err != nil {
			return
		}
		pCols = append(pCols, gf)
	}
	return
}

func normalizeColumn(col string) (gf string, err error) {
	if strings.Contains(col, ":") {
		gf, err = NormalizeGroupFunction(col)
		return
	}
	gf = col
	return
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

// GroupByClause get params in request to add group by clause
func GroupByClause(r *http.Request) (groupBySQL string) {
	queries := r.URL.Query()
	groupQuery := queries.Get("_groupby")
	if groupQuery == "" {
		return
	}

	if strings.Contains(groupQuery, "->>having") {
		params := strings.Split(groupQuery, ":")
		groupFieldQuery := strings.Split(groupQuery, "->>having")
		if len(params) != 5 {
			groupBySQL = fmt.Sprintf(statements.GroupBy, groupFieldQuery[0])
			return
		}
		// groupFunc, field, condition, conditionValue string
		groupFunc, err := NormalizeGroupFunction(fmt.Sprintf("%s:%s", params[1], params[2]))
		if err != nil {
			groupBySQL = fmt.Sprintf(statements.GroupBy, groupFieldQuery[0])
			return
		}

		operator, err := GetQueryOperator(params[3])
		if err != nil {
			groupBySQL = fmt.Sprintf(statements.GroupBy, groupFieldQuery[0])
			return
		}

		havingQuery := fmt.Sprintf(statements.Having, groupFunc, operator, params[4])
		groupBySQL = fmt.Sprintf("%s %s", fmt.Sprintf(statements.GroupBy, groupFieldQuery[0]), havingQuery)
		return
	}

	groupBySQL = fmt.Sprintf(statements.GroupBy, groupQuery)
	return
}

// NormalizeGroupFunction normalize url params values to sql group functions
func NormalizeGroupFunction(paramValue string) (groupFuncSQL string, err error) {
	values := strings.Split(paramValue, ":")
	groupFunc := strings.ToUpper(values[0])
	switch groupFunc {
	case "SUM", "AVG", "MAX", "MIN", "MEDIAN", "STDDEV", "VARIANCE":
		// values[1] it's a field in table
		groupFuncSQL = fmt.Sprintf("%s(%s)", groupFunc, values[1])
		return
	default:
		err = fmt.Errorf("this function %s is not a valid group function", groupFunc)
		return
	}
}
