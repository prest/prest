package postgres

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/lib/pq"

	"github.com/jmoiron/sqlx"
	"github.com/nuveo/log"
	"github.com/prest/adapters"
	"github.com/prest/adapters/postgres/formatters"
	"github.com/prest/adapters/postgres/internal/connection"
	"github.com/prest/adapters/postgres/statements"
	"github.com/prest/adapters/scanner"
	"github.com/prest/config"
)

//Postgres adapter postgresql
type Postgres struct {
}

const (
	pageNumberKey   = "_page"
	pageSizeKey     = "_page_size"
	defaultPageSize = 10
)

var removeOperatorRegex *regexp.Regexp
var insertTableNameQuotesRegex *regexp.Regexp
var insertTableNameRegex *regexp.Regexp
var groupRegex *regexp.Regexp

// ErrBodyEmpty err throw when body is empty
var ErrBodyEmpty = errors.New("body is empty")

var stmts *Stmt

// Stmt statement representation
type Stmt struct {
	Mtx        *sync.Mutex
	PrepareMap map[string]*sql.Stmt
}

// Prepare statement
func (s *Stmt) Prepare(db *sqlx.DB, tx *sql.Tx, SQL string) (statement *sql.Stmt, err error) {
	if config.PrestConf.EnableCache {
		var exists bool
		s.Mtx.Lock()
		statement, exists = s.PrepareMap[SQL]
		s.Mtx.Unlock()
		if exists {
			return
		}
	}

	if tx != nil {
		statement, err = tx.Prepare(SQL)
	} else {
		statement, err = db.Prepare(SQL)
	}

	if err != nil {
		return
	}
	if config.PrestConf.EnableCache {
		s.Mtx.Lock()
		s.PrepareMap[SQL] = statement
		s.Mtx.Unlock()
	}
	return
}

// Load postgres
func Load() {
	config.PrestConf.Adapter = &Postgres{}
	db, err := connection.Get()
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
}

func init() {
	removeOperatorRegex = regexp.MustCompile(`\$[a-z]+.`)
	insertTableNameRegex = regexp.MustCompile(`(?i)INTO\s+([\w|\.]*\.)*(\w+)\s*\(`)
	insertTableNameQuotesRegex = regexp.MustCompile(`(?i)INTO\s+([\w|\.|"]*\.)*"(\w+)"\s*\(`)
	groupRegex = regexp.MustCompile(`\"(.+?)\"`)
}

// GetStmt get statement
func GetStmt() *Stmt {
	if stmts == nil {
		stmts = &Stmt{
			Mtx:        &sync.Mutex{},
			PrepareMap: make(map[string]*sql.Stmt),
		}
	}
	return stmts
}

// ClearStmt used to reset the cache and allow multiple tests
func ClearStmt() {
	if stmts != nil {
		stmts = nil
		stmts = GetStmt()
	}
}

// GetTransaction get transaction
func (adapter *Postgres) GetTransaction() (tx *sql.Tx, err error) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		return
	}
	tx, err = db.Begin()
	return
}

// Prepare statement func
func Prepare(db *sqlx.DB, SQL string) (stmt *sql.Stmt, err error) {
	stmt, err = GetStmt().Prepare(db, nil, SQL)
	return
}

// PrepareTx statement func
func PrepareTx(tx *sql.Tx, SQL string) (stmt *sql.Stmt, err error) {
	stmt, err = GetStmt().Prepare(nil, tx, SQL)
	return
}

// chkInvalidIdentifier return true if identifier is invalid
func chkInvalidIdentifier(identifer ...string) bool {
	for _, ival := range identifer {
		if ival == "" || len(ival) > 63 || unicode.IsDigit([]rune(ival)[0]) {
			return true
		}
		count := 0
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
				v != ']' &&
				v != '"' {
				return true
			}
			if unicode.Is(unicode.Quotation_Mark, v) {
				count++
			}
		}
		if count%2 != 0 {
			return true
		}
	}
	return false
}

// WhereByRequest create interface for queries + where
func (adapter *Postgres) WhereByRequest(r *http.Request, initialPlaceholderID int) (whereSyntax string, values []interface{}, err error) {
	whereKey := []string{}
	whereValues := []string{}
	var value, op string

	pid := initialPlaceholderID
	for key, val := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {
			for k, v := range val {
				if v != "" {
					op = removeOperatorRegex.FindString(v)
					op = strings.Replace(op, ".", "", -1)
					if op == "" {
						op = "$eq"
					}
					value = removeOperatorRegex.ReplaceAllString(v, "")
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
						fields := strings.Split(jsonField[0], ".")
						jsonField[0] = fmt.Sprintf(`"%s"`, strings.Join(fields, `"."`))
						whereKey = append(whereKey, fmt.Sprintf(`%s->>'%s' %s $%d`, jsonField[0], jsonField[1], op, pid))
						values = append(values, value)
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

				if k == 0 {
					fields := strings.Split(key, ".")
					key = fmt.Sprintf(`"%s"`, strings.Join(fields, `"."`))
				}

				switch op {
				case "IN", "NOT IN":
					v := strings.Split(value, ",")
					keyParams := make([]string, len(v))
					for i := 0; i < len(v); i++ {
						whereValues = append(whereValues, v[i])
						keyParams[i] = fmt.Sprintf(`$%d`, pid+i)
					}
					pid += len(v)
					whereKey = append(whereKey, fmt.Sprintf(`%s %s (%s)`, key, op, strings.Join(keyParams, ",")))
				case "ANY", "SOME", "ALL":
					whereKey = append(whereKey, fmt.Sprintf(`%s = %s ($%d)`, key, op, pid))
					whereValues = append(whereValues, formatters.FormatArray(strings.Split(value, ",")))
					pid++
				case "IS NULL", "IS NOT NULL", "IS TRUE", "IS NOT TRUE", "IS FALSE", "IS NOT FALSE":
					whereKey = append(whereKey, fmt.Sprintf(`%s %s`, key, op))
				default: // "=", "!=", ">", ">=", "<", "<="
					whereKey = append(whereKey, fmt.Sprintf(`%s %s $%d`, key, op, pid))
					whereValues = append(whereValues, value)
					pid++
				}
			}
		}
	}

	for i := 0; i < len(whereKey); i++ {
		if whereSyntax == "" {
			whereSyntax += whereKey[i]
		} else {
			whereSyntax += " AND " + whereKey[i]
		}
	}

	for i := 0; i < len(whereValues); i++ {
		values = append(values, whereValues[i])
	}
	return
}

// ReturningByRequest create interface for queries + returning
func (adapter *Postgres) ReturningByRequest(r *http.Request) (returningSyntax string, err error) {
	queries := r.URL.Query()["_returning"]
	if len(queries) > 0 {
		for i, q := range queries {
			if i > 0 && i < len(queries) {
				returningSyntax += ", "
			}
			returningSyntax += q
		}
	}
	return
}

// SetByRequest create a set clause for SQL
func (adapter *Postgres) SetByRequest(r *http.Request, initialPlaceholderID int) (setSyntax string, values []interface{}, err error) {
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
		keys := strings.Split(key, ".")
		key = fmt.Sprintf(`"%s"`, strings.Join(keys, `"."`))
		fields = append(fields, fmt.Sprintf(`%s=$%d`, key, initialPlaceholderID))

		switch value.(type) {
		case []interface{}:
			values = append(values, formatters.FormatArray(value))
		default:
			values = append(values, value)
		}

		initialPlaceholderID++
	}
	setSyntax = strings.Join(fields, ", ")
	return
}

func closer(body io.Closer) {
	err := body.Close()
	if err != nil {
		log.Errorln(err)
	}
}

// ParseBatchInsertRequest create insert SQL to batch request
func (adapter *Postgres) ParseBatchInsertRequest(r *http.Request) (colsName string, placeholders string, values []interface{}, err error) {
	recordSet := make([]map[string]interface{}, 0)
	if err = json.NewDecoder(r.Body).Decode(&recordSet); err != nil {
		return
	}
	defer closer(r.Body)
	if len(recordSet) == 0 {
		err = ErrBodyEmpty
		return
	}
	recordKeys := adapter.tableKeys(recordSet[0])
	colsName = strings.Join(recordKeys, ",")
	values, placeholders, err = adapter.operationValues(recordSet, recordKeys)
	return
}

func (adapter *Postgres) operationValues(recordSet []map[string]interface{}, recordKeys []string) (values []interface{}, placeholders string, err error) {
	for i, record := range recordSet {
		initPH := len(values) + 1
		for _, key := range recordKeys {
			key, err = strconv.Unquote(key)
			if err != nil {
				return
			}
			value := record[key]
			switch value.(type) {
			case []interface{}:
				values = append(values, formatters.FormatArray(value))
			default:
				values = append(values, value)
			}
		}
		pl := adapter.createPlaceholders(initPH, len(values))
		placeholders = fmt.Sprintf("%s,%s", placeholders, pl)
		if i == 0 {
			placeholders = pl
		}
	}
	return
}

func (adapter *Postgres) tableKeys(json map[string]interface{}) (keys []string) {
	for key := range json {
		keys = append(keys, strconv.Quote(key))
	}
	sort.Strings(keys)
	return
}

func (adapter *Postgres) createPlaceholders(initial, lenValues int) (ret string) {
	for i := initial; i <= lenValues; i++ {
		if ret != "" {
			ret += ","
		}
		ret += fmt.Sprintf("$%d", i)
	}
	ret = fmt.Sprintf("(%s)", ret)
	return
}

// ParseInsertRequest create insert SQL
func (adapter *Postgres) ParseInsertRequest(r *http.Request) (colsName string, colsValue string, values []interface{}, err error) {
	body := make(map[string]interface{})
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		return
	}
	defer closer(r.Body)

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
		fields = append(fields, fmt.Sprintf(`"%s"`, key))

		switch value.(type) {
		case []interface{}:
			values = append(values, formatters.FormatArray(value))
		default:
			values = append(values, value)
		}
	}

	colsName = strings.Join(fields, ", ")
	colsValue = adapter.createPlaceholders(1, len(values))
	return
}

// DatabaseClause return a SELECT `query`
func (adapter *Postgres) DatabaseClause(req *http.Request) (query string, hasCount bool) {
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
func (adapter *Postgres) SchemaClause(req *http.Request) (query string, hasCount bool) {
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
func (adapter *Postgres) JoinByRequest(r *http.Request) (values []string, err error) {
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
	errJoin := errors.New("invalid join clause")
	if joinWith := strings.Split(joinArgs[1], "."); len(joinWith) == 2 {
		joinArgs[1] = fmt.Sprintf(`%s"."%s`, joinWith[0], joinWith[1])
	}
	spl := strings.Split(joinArgs[2], ".")
	if len(spl) != 2 {
		err = errJoin
		return
	}
	splj := strings.Split(joinArgs[4], ".")
	if len(splj) != 2 {
		err = errJoin
		return
	}
	joinQuery := fmt.Sprintf(` %s JOIN "%s" ON "%s"."%s" %s "%s"."%s" `, strings.ToUpper(joinArgs[0]), joinArgs[1], spl[0], spl[1], op, splj[0], splj[1])
	values = append(values, joinQuery)
	return
}

// SelectFields query
func (adapter *Postgres) SelectFields(fields []string) (sql string, err error) {
	if len(fields) == 0 {
		err = errors.New("you must select at least one field")
		return
	}
	var aux []string
	for _, field := range fields {
		if field != "*" && chkInvalidIdentifier(field) {
			err = fmt.Errorf("invalid identifier %s", field)
			return
		}
		if field != `*` {
			f := strings.Split(field, ".")

			isFunction, _ := regexp.MatchString(groupRegex.String(), field)
			if isFunction {
				aux = append(aux, strings.Join(f, `.`))
				continue
			}
			aux = append(aux, fmt.Sprintf(`"%s"`, strings.Join(f, `"."`)))
			continue
		}
		aux = append(aux, `*`)
	}
	sql = fmt.Sprintf("SELECT %s FROM", strings.Join(aux, ","))
	return
}

// OrderByRequest implements ORDER BY in queries
func (adapter *Postgres) OrderByRequest(r *http.Request) (values string, err error) {
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
			f := strings.Split(field, ".")
			field = fmt.Sprintf(`"%s"`, strings.Join(f, `"."`))
			if strings.HasPrefix(field, `"-`) {
				field = strings.Replace(field, `"-`, `"`, 1)
				field = fmt.Sprintf(`%s DESC`, field)
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
func (adapter *Postgres) CountByRequest(req *http.Request) (countQuery string, err error) {
	queries := req.URL.Query()
	countFields := queries.Get("_count")
	if countFields == "" {
		return
	}
	fields := strings.Split(countFields, ",")
	for i, field := range fields {
		if field != "*" && chkInvalidIdentifier(field) {
			err = errors.New("Invalid identifier")
			return
		}
		if field != `*` {
			f := strings.Split(field, ".")
			fields[i] = fmt.Sprintf(`"%s"`, strings.Join(f, `"."`))
		}
	}
	countQuery = fmt.Sprintf("SELECT COUNT(%s) FROM", strings.Join(fields, ","))
	return
}

// Query process queries
func (adapter *Postgres) Query(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	SQL = fmt.Sprintf("SELECT json_agg(s) FROM (%s) s", SQL)
	log.Debugln("generated SQL:", SQL, " parameters: ", params)
	p, err := Prepare(db, SQL)
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	var jsonData []byte
	err = p.QueryRow(params...).Scan(&jsonData)
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
func (adapter *Postgres) QueryCount(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	log.Debugln("generated SQL:", SQL, " parameters: ", params)
	p, err := Prepare(db, SQL)
	if err != nil {
		sc = &scanner.PrestScanner{Error: err}
		return
	}

	var result struct {
		Count int64 `json:"count"`
	}

	row := p.QueryRow(params...)
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
func (adapter *Postgres) PaginateIfPossible(r *http.Request) (paginatedQuery string, err error) {
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

// BatchInsertCopy execute batch insert sql into a table unsing copy
func (adapter *Postgres) BatchInsertCopy(dbname, schema, table string, keys []string, values ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	defer func() {
		var txerr error
		if err != nil {
			txerr = tx.Rollback()
			if txerr != nil {
				log.Errorln(txerr)
				return
			}
			return
		}
		txerr = tx.Commit()
		if txerr != nil {
			log.Errorln(txerr)
			return
		}
	}()
	for i := range keys {
		if strings.HasPrefix(keys[i], `"`) {
			keys[i], err = strconv.Unquote(keys[i])
			if err != nil {
				log.Println(err)
				sc = &scanner.PrestScanner{Error: err}
				return
			}
		}
	}
	stmt, err := tx.Prepare(pq.CopyInSchema(schema, table, keys...))
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	initOffSet := 0
	limitOffset := len(keys)
	for limitOffset <= len(values) {
		_, err = stmt.Exec(values[initOffSet:limitOffset]...)
		if err != nil {
			log.Println(err)
			sc = &scanner.PrestScanner{Error: err}
			return
		}
		initOffSet = limitOffset
		limitOffset += len(keys)
	}
	_, err = stmt.Exec()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	err = stmt.Close()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	sc = &scanner.PrestScanner{}
	return
}

// BatchInsertValues execute batch insert sql into a table unsing multi values
func (adapter *Postgres) BatchInsertValues(SQL string, values ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	stmt, err := adapter.fullInsert(db, nil, SQL)
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	jsonData := []byte("[")
	rows, err := stmt.Query(values...)
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	for rows.Next() {
		if err = rows.Err(); err != nil {
			if err != nil {
				log.Println(err)
				sc = &scanner.PrestScanner{Error: err}
				return
			}
		}
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			log.Println(err)
			sc = &scanner.PrestScanner{Error: err}
			return
		}
		if !bytes.Equal(jsonData, []byte("[")) {
			obj := fmt.Sprintf("%s,%s", jsonData, data)
			jsonData = []byte(obj)
			continue
		}
		jsonData = append(jsonData, data...)
	}
	jsonData = append(jsonData, byte(']'))
	sc = &scanner.PrestScanner{
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
	return
}

func (adapter *Postgres) fullInsert(db *sqlx.DB, tx *sql.Tx, SQL string) (stmt *sql.Stmt, err error) {
	tableName := insertTableNameQuotesRegex.FindStringSubmatch(SQL)
	if len(tableName) < 2 {
		tableName = insertTableNameRegex.FindStringSubmatch(SQL)
		if len(tableName) < 2 {
			err = errors.New("unable to find table name")
			return
		}
	}
	SQL = fmt.Sprintf(`%s RETURNING row_to_json("%s")`, SQL, tableName[2])
	if tx != nil {
		stmt, err = PrepareTx(tx, SQL)
	} else {
		stmt, err = Prepare(db, SQL)
	}
	return
}

// Insert execute insert sql into a table
func (adapter *Postgres) Insert(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	sc = adapter.insert(db, nil, SQL, params...)
	return
}

// InsertWithTransaction execute insert sql into a table
func (adapter *Postgres) InsertWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	sc = adapter.insert(nil, tx, SQL, params...)
	return
}

func (adapter *Postgres) insert(db *sqlx.DB, tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	stmt, err := adapter.fullInsert(db, tx, SQL)
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	log.Debugln(SQL, " parameters: ", params)
	var jsonData []byte
	err = stmt.QueryRow(params...).Scan(&jsonData)
	sc = &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
	return
}

// Delete execute delete sql into a table
func (adapter *Postgres) Delete(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	sc = adapter.delete(db, nil, SQL, params...)
	return
}

// DeleteWithTransaction execute delete sql into a table
func (adapter *Postgres) DeleteWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	sc = adapter.delete(nil, tx, SQL, params...)
	return
}

func (adapter *Postgres) delete(db *sqlx.DB, tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	log.Debugln("generated SQL:", SQL, " parameters: ", params)
	var stmt *sql.Stmt
	var err error
	if tx != nil {
		stmt, err = PrepareTx(tx, SQL)
	} else {
		stmt, err = Prepare(db, SQL)
	}
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", SQL, err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	if strings.Contains(SQL, "RETURNING") {
		rows, _ := stmt.Query(params...)
		cols, _ := rows.Columns()
		var data []map[string]interface{}
		for rows.Next() {
			columns := make([]interface{}, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				columnPointers[i] = &columns[i]
			}
			if err := rows.Scan(columnPointers...); err != nil {
				log.Fatal(err)
			}
			m := make(map[string]interface{})
			for i, colName := range cols {
				val := columnPointers[i].(*interface{})
				switch (*val).(type) {
				case []uint8:
					m[colName] = string((*val).([]byte))
				default:
					m[colName] = *val
				}
			}
			data = append(data, m)
		}
		jsonData, _ := json.Marshal(data)
		sc = &scanner.PrestScanner{
			Error: err,
			Buff:  bytes.NewBuffer(jsonData),
		}
		return
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

// Update execute update sql into a table
func (adapter *Postgres) Update(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	sc = adapter.update(db, nil, SQL, params...)
	return
}

// UpdateWithTransaction execute update sql into a table
func (adapter *Postgres) UpdateWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	sc = adapter.update(nil, tx, SQL, params...)
	return
}

func (adapter *Postgres) update(db *sqlx.DB, tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	var stmt *sql.Stmt
	var err error
	if tx != nil {
		stmt, err = PrepareTx(tx, SQL)
	} else {
		stmt, err = Prepare(db, SQL)
	}
	if err != nil {
		log.Printf("could not prepare sql: %s\n Error: %v\n", SQL, err)
		sc = &scanner.PrestScanner{Error: err}
		return
	}
	log.Debugln("generated SQL:", SQL, " parameters: ", params)
	if strings.Contains(SQL, "RETURNING") {
		rows, _ := stmt.Query(params...)
		cols, _ := rows.Columns()
		var data []map[string]interface{}
		for rows.Next() {
			columns := make([]interface{}, len(cols))
			columnPointers := make([]interface{}, len(cols))
			for i := range columns {
				columnPointers[i] = &columns[i]
			}
			if err := rows.Scan(columnPointers...); err != nil {
				log.Fatal(err)
			}
			m := make(map[string]interface{})
			for i, colName := range cols {
				val := columnPointers[i].(*interface{})
				switch (*val).(type) {
				case []uint8:
					m[colName] = string((*val).([]byte))
				default:
					m[colName] = *val
				}
			}
			data = append(data, m)
		}
		jsonData, _ := json.Marshal(data)
		sc = &scanner.PrestScanner{
			Error: err,
			Buff:  bytes.NewBuffer(jsonData),
		}
		return
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
	case "any":
		return "ANY", nil
	case "some":
		return "SOME", nil
	case "all":
		return "ALL", nil
	case "notnull":
		return "IS NOT NULL", nil
	case "null":
		return "IS NULL", nil
	case "true":
		return "IS TRUE", nil
	case "nottrue":
		return "IS NOT TRUE", nil
	case "false":
		return "IS FALSE", nil
	case "notfalse":
		return "IS NOT FALSE", nil
	case "like":
		return "LIKE", nil
	case "ilike":
		return "ILIKE", nil
	}

	err := errors.New("Invalid operator")
	return "", err

}

// TablePermissions get tables permissions based in prest configuration
func (adapter *Postgres) TablePermissions(table string, op string) bool {
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

func fieldsByPermission(table, op string) (fields []string) {
	tables := config.PrestConf.AccessConf.Tables
	for _, t := range tables {
		if t.Name == table {
			for _, perm := range t.Permissions {
				if perm == op {
					fields = t.Fields
				}
			}
		}
	}
	return
}

func containsAsterisk(arr []string) bool {
	for _, e := range arr {
		if e == "*" {
			return true
		}
	}
	return false
}

func intersection(set, other []string) (intersection []string) {
	for _, field := range set {
		pField := checkField(field, other)
		if pField != "" {
			intersection = append(intersection, pField)
		}
	}
	return
}

// FieldsPermissions get fields permissions based in prest configuration
func (adapter *Postgres) FieldsPermissions(r *http.Request, table string, op string) (fields []string, err error) {
	cols, err := columnsByRequest(r)
	if err != nil {
		err = fmt.Errorf("error on parse columns from request: %s", err)
		return
	}
	restrict := config.PrestConf.AccessConf.Restrict
	if !restrict || op == "delete" {
		if len(cols) > 0 {
			fields = cols
			return
		}
		fields = []string{"*"}
		return
	}
	allowedFields := fieldsByPermission(table, op)
	if len(allowedFields) == 0 {
		err = errors.New("there's no configured field for this table")
		return
	}
	if containsAsterisk(allowedFields) {
		fields = []string{"*"}
		if len(cols) > 0 {
			fields = cols
		}
		return
	}
	fields = intersection(cols, allowedFields)
	if len(cols) == 0 {
		fields = allowedFields
	}
	return
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

// columnsByRequest extract columns and return as array of strings
func columnsByRequest(r *http.Request) (columns []string, err error) {
	queries := r.URL.Query()
	columnsArr := queries["_select"]
	for _, j := range columnsArr {
		cArgs := strings.Split(j, ",")
		for _, columnName := range cArgs {
			columns = append(columns, columnName)
		}
	}
	if queries.Get("_groupby") != "" {
		columns, err = normalizeAll(columns)
		if err != nil {
			return
		}
	}
	return
}

// DistinctClause get params in request to add distinct clause
func (adapter *Postgres) DistinctClause(r *http.Request) (distinctQuery string, err error) {
	queries := r.URL.Query()
	checkQuery := queries.Get("_distinct")
	distinctQuery = ""

	if checkQuery == "true" {
		distinctQuery = fmt.Sprintf("SELECT DISTINCT")
	}
	return
}

// GroupByClause get params in request to add group by clause
func (adapter *Postgres) GroupByClause(r *http.Request) (groupBySQL string) {
	queries := r.URL.Query()
	groupQuery := queries.Get("_groupby")
	if groupQuery == "" {
		return
	}

	if strings.Contains(groupQuery, "->>having") {
		params := strings.Split(groupQuery, ":")
		groupFieldQuery := strings.Split(groupQuery, "->>having")

		fields := strings.Split(groupFieldQuery[0], ",")
		for i, field := range fields {
			f := strings.Split(field, ".")
			fields[i] = fmt.Sprintf(`"%s"`, strings.Join(f, `"."`))
		}
		groupFieldQuery[0] = strings.Join(fields, ",")
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
	fields := strings.Split(groupQuery, ",")
	for i, field := range fields {
		f := strings.Split(field, ".")
		fields[i] = fmt.Sprintf(`"%s"`, strings.Join(f, `"."`))
	}
	groupQuery = strings.Join(fields, ",")
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
		v := values[1]
		if v != "*" {
			values[1] = fmt.Sprintf(`"%s"`, v)
		}
		groupFuncSQL = fmt.Sprintf(`%s(%s)`, groupFunc, values[1])
		return
	default:
		err = fmt.Errorf("this function %s is not a valid group function", groupFunc)
		return
	}
}

// SetDatabase set the current database name in use
func (adapter *Postgres) SetDatabase(name string) {
	connection.SetDatabase(name)
}

// SelectSQL generate select sql
func (adapter *Postgres) SelectSQL(selectStr string, database string, schema string, table string) string {
	return fmt.Sprintf(`%s "%s"."%s"."%s"`, selectStr, database, schema, table)
}

// InsertSQL generate insert sql
func (adapter *Postgres) InsertSQL(database string, schema string, table string, names string, placeholders string) string {
	return fmt.Sprintf(statements.InsertQuery, database, schema, table, names, placeholders)
}

// DeleteSQL generate delete sql
func (adapter *Postgres) DeleteSQL(database string, schema string, table string) string {
	return fmt.Sprintf(statements.DeleteQuery, database, schema, table)
}

// UpdateSQL generate update sql
func (adapter *Postgres) UpdateSQL(database string, schema string, table string, setSyntax string) string {
	return fmt.Sprintf(statements.UpdateQuery, database, schema, table, setSyntax)
}

// DatabaseWhere generate database where syntax
func (adapter *Postgres) DatabaseWhere(requestWhere string) (whereSyntax string) {
	whereSyntax = statements.DatabasesWhere
	if requestWhere != "" {
		whereSyntax = fmt.Sprint(whereSyntax, " AND ", requestWhere)
	}
	return
}

// DatabaseOrderBy generate database order by
func (adapter *Postgres) DatabaseOrderBy(order string, hasCount bool) (orderBy string) {
	if order != "" {
		orderBy = order
	} else if !hasCount {
		orderBy = fmt.Sprintf(statements.DatabasesOrderBy, statements.FieldDatabaseName)
	}
	return
}

// SchemaOrderBy generate schema order by
func (adapter *Postgres) SchemaOrderBy(order string, hasCount bool) (orderBy string) {
	if order != "" {
		orderBy = order
	} else if !hasCount {
		orderBy = fmt.Sprintf(statements.SchemasOrderBy, statements.FieldSchemaName)
	}
	return
}

// TableClause generate table clause
func (adapter *Postgres) TableClause() (query string) {
	query = statements.TablesSelect
	return
}

// TableWhere generate table where syntax
func (adapter *Postgres) TableWhere(requestWhere string) (whereSyntax string) {
	whereSyntax = statements.TablesWhere
	if requestWhere != "" {
		whereSyntax = fmt.Sprint(whereSyntax, " AND ", requestWhere)
	}
	return
}

// TableOrderBy generate table order by
func (adapter *Postgres) TableOrderBy(order string) (orderBy string) {
	if order != "" {
		orderBy = order
	} else {
		orderBy = statements.TablesOrderBy
	}
	return
}

// SchemaTablesClause generate schema tables clause
func (adapter *Postgres) SchemaTablesClause() (query string) {
	query = statements.SchemaTablesSelect
	return
}

// SchemaTablesWhere generate schema tables where syntax
func (adapter *Postgres) SchemaTablesWhere(requestWhere string) (whereSyntax string) {
	whereSyntax = statements.SchemaTablesWhere
	if requestWhere != "" {
		whereSyntax = fmt.Sprint(whereSyntax, " AND ", requestWhere)
	}
	return
}

// SchemaTablesOrderBy generate schema tables order by
func (adapter *Postgres) SchemaTablesOrderBy(order string) (orderBy string) {
	if order != "" {
		orderBy = order
	} else {
		orderBy = statements.SchemaTablesOrderBy
	}
	return
}
