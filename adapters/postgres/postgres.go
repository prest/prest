package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/prest/prest/v2/adapters"
	"github.com/prest/prest/v2/adapters/postgres/formatters"
	"github.com/prest/prest/v2/adapters/postgres/internal/connection"
	"github.com/prest/prest/v2/adapters/postgres/statements"
	"github.com/prest/prest/v2/adapters/scanner"
	"github.com/prest/prest/v2/config"
	pctx "github.com/prest/prest/v2/context"
	"github.com/prest/prest/v2/internal/ident"
	"github.com/prest/prest/v2/template"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/structy/log"
)

// Postgres adapter postgresql
type Postgres struct{}

const (
	pageNumberKey   = "_page"
	pageSizeKey     = "_page_size"
	defaultPageSize = 10
	//nolint
	defaultPageNumber = 1
)

var removeOperatorRegex *regexp.Regexp
var insertTableNameQuotesRegex *regexp.Regexp
var insertTableNameRegex *regexp.Regexp
var groupRegex *regexp.Regexp

var stmts *Stmt

// Stmt statement representation
type Stmt struct {
	Mtx        *sync.Mutex
	PrepareMap map[string]*sql.Stmt
}

// Prepare statement
func (s *Stmt) Prepare(db *sqlx.DB, tx *sql.Tx, SQL string) (statement *sql.Stmt, err error) {
	if config.PrestConf.PGCache && (tx == nil) {
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
	if config.PrestConf.PGCache && (tx == nil) {
		s.Mtx.Lock()
		s.PrepareMap[SQL] = statement
		s.Mtx.Unlock()
	}
	return
}

// Load postgres
func Load() {
	config.PrestConf.Adapter = &Postgres{}

	if connection.GetDatabase() == "" {
		connection.SetDatabase(config.PrestConf.PGDatabase)
	}

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
	insertTableNameRegex = regexp.MustCompile(`(?i)INTO\s+([\w|\.|-]*\.)*([\w|-]+)\s*\(`)
	insertTableNameQuotesRegex = regexp.MustCompile(`(?i)INTO\s+([\w|\.|"|-]*\.)*"([\w|-]+)"\s*\(`)
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
	return db.Begin()
}

// GetTransactionCtx get transaction
func (adapter *Postgres) GetTransactionCtx(ctx context.Context) (tx *sql.Tx, err error) {
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return
	}
	return db.Begin()
}

// Prepare statement func
func Prepare(db *sqlx.DB, SQL string) (stmt *sql.Stmt, err error) {
	return GetStmt().Prepare(db, nil, SQL)
}

// PrepareTx statement func
func PrepareTx(tx *sql.Tx, SQL string) (stmt *sql.Stmt, err error) {
	return GetStmt().Prepare(nil, tx, SQL)
}

// chkInvalidIdentifier return true if identifier is invalid
func chkInvalidIdentifier(identifer ...string) bool {
	for _, ival := range identifer {
		if ival == "" || unicode.IsDigit([]rune(ival)[0]) {
			return true
		}

		ivalSplit := strings.Split(ival, ".")
		if len(ivalSplit) == 2 && len(ivalSplit[len(ivalSplit)-1]) > 63 {
			return true
		}

		if !strings.Contains(ival, ".") && len(ival) > 63 {
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
			// keep the original key untouched to avoid invalid identifier errors
			rawKey := key
			for _, v := range val {
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

				keyInfo := strings.Split(rawKey, ":")

				if len(keyInfo) > 1 {
					switch keyInfo[1] {
					case "jsonb":
						jsonField := strings.Split(keyInfo[0], "->>")
						if len(jsonField) != 2 || !ident.IsValid(jsonField[0]) || !ident.IsValid(jsonField[1]) {
							err = errors.Wrapf(ErrInvalidIdentifier, "%v", jsonField)
							return
						}
						fields := strings.Split(jsonField[0], ".")
						jsonField[0] = fmt.Sprintf(`"%s"`, strings.Join(fields, `"."`))
						// escape single quotes in json attribute key
						safeAttr := strings.ReplaceAll(jsonField[1], "'", "''")
						whereKey = append(whereKey, fmt.Sprintf(`%s->>'%s' %s $%d`, jsonField[0], safeAttr, op, pid))
						values = append(values, value)
					case "tsquery":
						tsQueryField := strings.Split(keyInfo[0], "$")
						tsQuery := fmt.Sprintf(`%s @@ to_tsquery('%s')`, tsQueryField[0], value)
						if len(tsQueryField) == 2 {
							tsQuery = fmt.Sprintf(`%s @@ to_tsquery('%s', '%s')`, tsQueryField[0], tsQueryField[1], value)
						}
						whereKey = append(whereKey, tsQuery)
					default:
						if !ident.IsValid(keyInfo[0]) {
							err = errors.Wrapf(ErrInvalidIdentifier, "%s", keyInfo[0])
							return
						}
					}
					pid++
					continue
				}

				if !ident.IsValid(rawKey) {
					err = errors.Wrapf(ErrInvalidIdentifier, "%s", rawKey)
					return
				}

				// always quote the field for SQL usage without mutating the original key
				fields := strings.Split(rawKey, ".")
				quotedKey := fmt.Sprintf(`"%s"`, strings.Join(fields, `"."`))

				switch op {
				case "IN", "NOT IN":
					v := strings.Split(value, ",")
					keyParams := make([]string, len(v))
					for i := 0; i < len(v); i++ {
						whereValues = append(whereValues, v[i])
						keyParams[i] = fmt.Sprintf(`$%d`, pid+i)
					}
					pid += len(v)
					whereKey = append(whereKey, fmt.Sprintf(`%s %s (%s)`, quotedKey, op, strings.Join(keyParams, ",")))
				case "ANY", "SOME", "ALL":
					whereKey = append(whereKey, fmt.Sprintf(`%s = %s ($%d)`, quotedKey, op, pid))
					whereValues = append(whereValues, formatters.FormatArray(strings.Split(value, ",")))
					pid++
				case "IS NULL", "IS NOT NULL", "IS TRUE", "IS NOT TRUE", "IS FALSE", "IS NOT FALSE":
					whereKey = append(whereKey, fmt.Sprintf(`%s %s`, quotedKey, op))
				default: // "=", "!=", ">", ">=", "<", "<="
					whereKey = append(whereKey, fmt.Sprintf(`%s %s $%d`, quotedKey, op, pid))
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
	// TODO: write documentation:
	// https://docs.prestd.com/api-reference/parameters
	queries := r.URL.Query()["_returning"]
	if len(queries) > 0 {
		cols := make([]string, 0, len(queries))
		for _, q := range queries {
			if q == "*" {
				cols = append(cols, "*")
				continue
			}
			if !ident.IsValid(q) {
				err = errors.Wrap(ErrInvalidIdentifier, "Returning")
				return
			}
			quoted, _ := ident.Quote(q)
			cols = append(cols, quoted)
		}
		returningSyntax = strings.Join(cols, ", ")
	}
	return
}

func sliceToJSONList(ifaceSlice interface{}) (returnValue string, err error) {
	v := reflect.ValueOf(ifaceSlice)

	if v.Kind() == reflect.Invalid {
		return "[]", ErrEmptyOrInvalidSlice
	}

	value := make([]string, 0)

	for i := 0; i < v.Len(); i++ {
		val := v.Index(i).Interface()
		switch val.(type) {
		case int, float64:
			newVal := fmt.Sprint(val)
			value = append(value, newVal)
		default:
			newVal := fmt.Sprintf(`"%s"`, val)
			value = append(value, newVal)
		}
	}
	returnValue = fmt.Sprintf(`[%v]`, strings.Join(value, ", "))
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
		if !ident.IsValid(key) {
			err = errors.Wrap(ErrInvalidIdentifier, "Set")
			return
		}
		keys := strings.Split(key, ".")
		key = fmt.Sprintf(`"%s"`, strings.Join(keys, `"."`))
		fields = append(fields, fmt.Sprintf(`%s=$%d`, key, initialPlaceholderID))

		switch reflect.ValueOf(value).Kind() {
		case reflect.Interface:
			values = append(values, formatters.FormatArray(value))
		case reflect.Map:
			jsonData, err := json.Marshal(value)
			if err != nil {
				log.Errorln(err)
			}
			values = append(values, string(jsonData))
		case reflect.Slice:
			value, err = sliceToJSONList(value)
			if err != nil {
				log.Errorln(err)
			}
			values = append(values, value)
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
		if !ident.IsValid(key) {
			err = errors.Wrap(ErrInvalidIdentifier, "Insert")
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

	query = fmt.Sprintf(statements.DatabasesSelect, statements.FieldDatabaseName)
	if countQuery != "" {
		hasCount = true
		query = fmt.Sprintf(statements.DatabasesSelect, statements.FieldCountDatabaseName)
	}
	return
}

// SchemaClause return a SELECT `query`
func (adapter *Postgres) SchemaClause(req *http.Request) (query string, hasCount bool) {
	queries := req.URL.Query()
	countQuery := queries.Get("_count")

	query = fmt.Sprintf(statements.SchemasSelect, statements.FieldSchemaName)
	if countQuery != "" {
		hasCount = true
		query = fmt.Sprintf(statements.SchemasSelect, statements.FieldCountSchemaName)
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
		err = ErrJoinInvalidNumberOfArgs
		return
	}

	// whitelist join types
	jt := strings.ToUpper(joinArgs[0])
	allowed := map[string]bool{"INNER": true, "LEFT": true, "RIGHT": true, "FULL": true, "CROSS": true}
	if !allowed[jt] {
		err = ErrInvalidJoinClause
		return
	}

	if !ident.IsValid(joinArgs[1]) || !ident.IsValid(joinArgs[2]) || !ident.IsValid(joinArgs[4]) {
		err = ErrInvalidIdentifier
		return
	}

	op, err := GetQueryOperator(joinArgs[3])
	if err != nil {
		return
	}
	errJoin := ErrInvalidJoinClause
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
	joinQuery := fmt.Sprintf(` %s JOIN "%s" ON "%s"."%s" %s "%s"."%s" `, jt, joinArgs[1], spl[0], spl[1], op, splj[0], splj[1])
	values = append(values, joinQuery)
	return
}

// SelectFields query
func (adapter *Postgres) SelectFields(fields []string) (sql string, err error) {
	if len(fields) == 0 {
		err = ErrMustSelectOneField
		return
	}
	var aux []string

	for _, field := range fields {
		groupFunc, _ := NormalizeGroupFunction(field)

		if groupFunc != "" {
			aux = append(aux, groupFunc)
			continue
		}

		if field != `*` {
			// Allow function-like expressions already quoted, e.g., SUM("salary")
			isFunction, _ := regexp.MatchString(groupRegex.String(), field)
			if isFunction {
				aux = append(aux, field)
				continue
			}
			if !ident.IsValid(field) {
				err = errors.Wrapf(ErrInvalidIdentifier, "%s", field)
				return
			}
			q, _ := ident.Quote(field)
			aux = append(aux, q)
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

		for i, fld := range orderingArr {
			desc := false
			field := fld
			if strings.HasPrefix(field, "-") {
				desc = true
				field = field[1:]
			}
			if !ident.IsValid(field) {
				err = ErrInvalidIdentifier
				values = ""
				return
			}
			q, _ := ident.Quote(field)
			if desc {
				q = fmt.Sprintf("%s DESC", q)
			}
			values = fmt.Sprintf("%s %s", values, q)
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
	selectFields := queries.Get("_select")
	if countFields == "" {
		return
	}
	if selectFields != "" {
		selectFields = fmt.Sprintf(", %s", selectFields)
	}
	fields := strings.Split(countFields, ",")
	for i, field := range fields {
		if field != "*" && !ident.IsValid(field) {
			err = ErrInvalidIdentifier
			return
		}
		if field != `*` {
			q, _ := ident.Quote(field)
			fields[i] = q
		}
	}
	countQuery = fmt.Sprintf("SELECT COUNT(%s)%s FROM", strings.Join(fields, ","), selectFields)
	return
}

// QueryCtx process queries using the DB name from Context
//
// allows setting timeout
func (adapter *Postgres) QueryCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	// use the db_name that was set on request to avoid runtime collisions
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	SQL = fmt.Sprintf("SELECT %s(s) FROM (%s) s", config.PrestConf.JSONAggType, SQL)
	log.Debugln("generated SQL:", SQL, " parameters: ", params)
	p, err := Prepare(db, SQL)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	var jsonData []byte
	err = p.QueryRowContext(ctx, params...).Scan(&jsonData)
	if len(jsonData) == 0 {
		jsonData = []byte("[]")
	}
	return &scanner.PrestScanner{
		Error:   err,
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
}

func (adapter *Postgres) Query(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Println(err)
		return &scanner.PrestScanner{Error: err}
	}
	SQL = fmt.Sprintf("SELECT %s(s) FROM (%s) s", config.PrestConf.JSONAggType, SQL)
	log.Debugln("generated SQL:", SQL, " parameters: ", params)
	p, err := Prepare(db, SQL)
	if err != nil {
		return &scanner.PrestScanner{Error: err}
	}
	var jsonData []byte
	err = p.QueryRow(params...).Scan(&jsonData)
	if len(jsonData) == 0 {
		jsonData = []byte("[]")
	}
	return &scanner.PrestScanner{
		Error:   err,
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
}

// QueryCount process queries with count
func (adapter *Postgres) QueryCount(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		return &scanner.PrestScanner{Error: err}
	}

	log.Debugln("generated SQL:", SQL, " parameters: ", params)
	p, err := Prepare(db, SQL)
	if err != nil {
		return &scanner.PrestScanner{Error: err}
	}

	var result struct {
		Count int64 `json:"count"`
	}

	row := p.QueryRow(params...)
	if err = row.Scan(&result.Count); err != nil {
		return &scanner.PrestScanner{Error: err}
	}
	var byt []byte
	byt, err = json.Marshal(result)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(byt),
	}
}

// QueryCount process queries with count
func (adapter *Postgres) QueryCountCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	log.Debugln("generated SQL:", SQL, " parameters: ", params)
	p, err := Prepare(db, SQL)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}

	var result struct {
		Count int64 `json:"count"`
	}

	row := p.QueryRow(params...)
	if err = row.Scan(&result.Count); err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	var byt []byte
	byt, err = json.Marshal(result)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(byt),
	}
}

// PaginateIfPossible when passing non-valid paging parameters (conversion to integer) the query will be made with default value
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
	return template.LimitOffset(fmt.Sprint(pageNumber), fmt.Sprint(pageSize))
}

// BatchInsertCopy execute batch insert sql into a table unsing copy
func (adapter *Postgres) BatchInsertCopy(dbname, schema, table string, keys []string, values ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	tx, err := db.Begin()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
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
				log.Errorln(err)
				return &scanner.PrestScanner{Error: err}
			}
		}
	}
	stmt, err := tx.Prepare(pq.CopyInSchema(schema, table, keys...))
	if err != nil {
		log.Println(err)
		return &scanner.PrestScanner{Error: err}
	}
	initOffSet := 0
	limitOffset := len(keys)
	for limitOffset <= len(values) {
		_, err = stmt.Exec(values[initOffSet:limitOffset]...)
		if err != nil {
			log.Errorln(err)
			return &scanner.PrestScanner{Error: err}
		}
		initOffSet = limitOffset
		limitOffset += len(keys)
	}
	_, err = stmt.Exec()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	err = stmt.Close()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	return &scanner.PrestScanner{}
}

// BatchInsertCopyCtx execute batch insert sql into a table unsing copy
func (adapter *Postgres) BatchInsertCopyCtx(ctx context.Context, dbname, schema, table string, keys []string, values ...interface{}) (sc adapters.Scanner) {
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	tx, err := db.Begin()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
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
				log.Errorln(err)
				return &scanner.PrestScanner{Error: err}
			}
		}
	}
	stmt, err := tx.Prepare(pq.CopyInSchema(schema, table, keys...))
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	initOffSet := 0
	limitOffset := len(keys)
	for limitOffset <= len(values) {
		_, err = stmt.Exec(values[initOffSet:limitOffset]...)
		if err != nil {
			log.Errorln(err)
			return &scanner.PrestScanner{Error: err}
		}
		initOffSet = limitOffset
		limitOffset += len(keys)
	}
	_, err = stmt.Exec()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	err = stmt.Close()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	return &scanner.PrestScanner{}
}

// BatchInsertValues execute batch insert sql into a table unsing multi values
func (adapter *Postgres) BatchInsertValues(SQL string, values ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	stmt, err := adapter.fullInsert(db, nil, SQL)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	jsonData := []byte("[")
	rows, err := stmt.Query(values...)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	for rows.Next() {
		if err = rows.Err(); err != nil {
			log.Errorln(err)
			return &scanner.PrestScanner{Error: err}
		}
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			log.Errorln(err)
			return &scanner.PrestScanner{Error: err}
		}
		if !bytes.Equal(jsonData, []byte("[")) {
			obj := fmt.Sprintf("%s,%s", jsonData, data)
			jsonData = []byte(obj)
			continue
		}
		jsonData = append(jsonData, data...)
	}
	jsonData = append(jsonData, byte(']'))
	return &scanner.PrestScanner{
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
}

// BatchInsertValuesCtx execute batch insert sql into a table unsing multi values
func (adapter *Postgres) BatchInsertValuesCtx(ctx context.Context, SQL string, values ...interface{}) (sc adapters.Scanner) {
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	stmt, err := adapter.fullInsert(db, nil, SQL)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	jsonData := []byte("[")
	rows, err := stmt.Query(values...)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	for rows.Next() {
		if err = rows.Err(); err != nil {
			log.Errorln(err)
			return &scanner.PrestScanner{Error: err}
		}
		var data []byte
		err = rows.Scan(&data)
		if err != nil {
			log.Errorln(err)
			return &scanner.PrestScanner{Error: err}
		}
		if !bytes.Equal(jsonData, []byte("[")) {
			obj := fmt.Sprintf("%s,%s", jsonData, data)
			jsonData = []byte(obj)
			continue
		}
		jsonData = append(jsonData, data...)
	}
	jsonData = append(jsonData, byte(']'))
	return &scanner.PrestScanner{
		Buff:    bytes.NewBuffer(jsonData),
		IsQuery: true,
	}
}

func (adapter *Postgres) fullInsert(db *sqlx.DB, tx *sql.Tx, SQL string) (stmt *sql.Stmt, err error) {
	tableName := insertTableNameQuotesRegex.FindStringSubmatch(SQL)
	if len(tableName) < 2 {
		tableName = insertTableNameRegex.FindStringSubmatch(SQL)
		if len(tableName) < 2 {
			err = ErrNoTableName
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
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.insert(db, nil, SQL, params...)
}

// InsertCtx execute insert sql into a table
func (adapter *Postgres) InsertCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.insert(db, nil, SQL, params...)
}

// InsertWithTransaction execute insert sql into a table
func (adapter *Postgres) InsertWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	return adapter.insert(nil, tx, SQL, params...)
}

func (adapter *Postgres) insert(db *sqlx.DB, tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	stmt, err := adapter.fullInsert(db, tx, SQL)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	log.Debugln(SQL, " parameters: ", params)
	var jsonData []byte
	err = stmt.QueryRow(params...).Scan(&jsonData)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
}

// Delete execute delete sql into a table
func (adapter *Postgres) Delete(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.delete(db, nil, SQL, params...)
}

// Delete execute delete sql into a table
func (adapter *Postgres) DeleteCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.delete(db, nil, SQL, params...)
}

// DeleteWithTransaction execute delete sql into a table
func (adapter *Postgres) DeleteWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	return adapter.delete(nil, tx, SQL, params...)
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
		return &scanner.PrestScanner{Error: err}
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
		return &scanner.PrestScanner{
			Error: err,
			Buff:  bytes.NewBuffer(jsonData),
		}
	}
	var result sql.Result
	var rowsAffected int64
	result, err = stmt.Exec(params...)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var jsonData []byte
	jsonData, err = json.Marshal(data)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
}

// Update execute update sql into a table
func (adapter *Postgres) Update(SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := connection.Get()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.update(db, nil, SQL, params...)
}

// Update execute update sql into a table
func (adapter *Postgres) UpdateCtx(ctx context.Context, SQL string, params ...interface{}) (sc adapters.Scanner) {
	db, err := getDBFromCtx(ctx)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	return adapter.update(db, nil, SQL, params...)
}

// UpdateWithTransaction execute update sql into a table
func (adapter *Postgres) UpdateWithTransaction(tx *sql.Tx, SQL string, params ...interface{}) (sc adapters.Scanner) {
	return adapter.update(nil, tx, SQL, params...)
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
		log.Errorf("could not prepare sql: %s\n Error: %v\n", SQL, err)
		return &scanner.PrestScanner{Error: err}
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
		return &scanner.PrestScanner{
			Error: err,
			Buff:  bytes.NewBuffer(jsonData),
		}
	}
	var result sql.Result
	var rowsAffected int64
	result, err = stmt.Exec(params...)
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		log.Errorln(err)
		return &scanner.PrestScanner{Error: err}
	}
	data := make(map[string]interface{})
	data["rows_affected"] = rowsAffected
	var jsonData []byte
	jsonData, err = json.Marshal(data)
	return &scanner.PrestScanner{
		Error: err,
		Buff:  bytes.NewBuffer(jsonData),
	}
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
	case "nlike":
		return "NOT LIKE", nil
	case "nilike":
		return "NOT ILIKE", nil
	// ltree features
	case "ltreelanc":
		return "@>", nil
	case "ltreerdesc":
		return "<@", nil
	case "ltreematch":
		return "~", nil
	case "ltreematchtxt":
		return "@", nil
	}

	return "", ErrInvalidOperator
}

// TablePermissions get tables permissions based in prest configuration
func (adapter *Postgres) TablePermissions(table string, op string, userName string) (access bool) {
	restrict := config.PrestConf.AccessConf.Restrict
	if !restrict {
		return true
	}

	// ignore table loop
	for _, ignoreT := range config.PrestConf.AccessConf.IgnoreTable {
		if ignoreT == table {
			return true
		}
	}

	tables := config.PrestConf.AccessConf.Tables
	access = false
	for _, t := range tables {
		if t.Name == table {
			access = slices.Contains(t.Permissions, op)
			break
		}
	}

	// If userName is empty, means use table access.
	if userName == "" {
		return access
	}

	// currently, access is granted to all users based on the table settings.
	// if it is later discovered that there are specific permission settings for an individual user,
	// then the latter settings should be applied.
	users := config.PrestConf.AccessConf.Users
	for _, u := range users {
		if u.Name == userName {
			for _, t := range u.Tables {
				if t.Name == table {
					return slices.Contains(t.Permissions, op)
				}
			}
		}
	}
	return access
}

// fieldsByPermission returns a list of fields that a user is allowed to access
// for a given table and operation based on the configuration.
//
// Parameters:
//   - table: The name of the table to check permissions for.
//   - operation: The type of operation (e.g., "read", "write") to check permissions for.
//   - userName: The name of the user to check permissions for.
//
// Returns:
//   - fields: A slice of strings representing the fields the user is allowed to access.
//     If no specific permissions are found, it defaults to returning all fields ("*").
func fieldsByPermission(table, operation, userName string) (fields []string) {
	fields = []string{"*"}
	confTables := config.PrestConf.AccessConf.Tables

	for _, cfgTable := range confTables {
		if cfgTable.Name == table {
			for _, perm := range cfgTable.Permissions {
				if perm == operation {
					fields = cfgTable.Fields
				}
			}
		}
	}

	if userName == "" {
		return
	}

	// individual user
	users := config.PrestConf.AccessConf.Users
	for _, u := range users {
		if u.Name == userName {
			for _, t := range u.Tables {
				if t.Name == table &&
					slices.Contains(t.Permissions, operation) {
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
func (adapter *Postgres) FieldsPermissions(r *http.Request, table string, op string, userName string) (fields []string, err error) {
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
	allowedFields := fieldsByPermission(table, op, userName)
	if containsAsterisk(allowedFields) {
		fields = []string{"*"}
		if len(cols) > 0 {
			fields = cols
		}
		return
	}
	fields = intersection(cols, allowedFields)
	if len(cols) == 0 && len(allowedFields) > 0 {
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
		columns = append(columns, cArgs...)
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
		distinctQuery = "SELECT DISTINCT"
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
			if !ident.IsValid(field) {
				return ""
			}
			q, _ := ident.Quote(field)
			fields[i] = q
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

		// sanitize having value: numeric stays raw, string gets single-quoted and escaped
		val := params[4]
		if _, errNum := strconv.ParseFloat(val, 64); errNum == nil {
			havingQuery := fmt.Sprintf(statements.Having, groupFunc, operator, val)
			groupBySQL = fmt.Sprintf("%s %s", fmt.Sprintf(statements.GroupBy, groupFieldQuery[0]), havingQuery)
			return
		}
		safe := strings.ReplaceAll(val, "'", "''")
		havingQuery := fmt.Sprintf(statements.Having, groupFunc, operator, fmt.Sprintf("'%s'", safe))
		groupBySQL = fmt.Sprintf("%s %s", fmt.Sprintf(statements.GroupBy, groupFieldQuery[0]), havingQuery)
		return
	}
	fields := strings.Split(groupQuery, ",")
	for i, field := range fields {
		if !ident.IsValid(field) {
			return ""
		}
		q, _ := ident.Quote(field)
		fields[i] = q
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
	case "SUM", "AVG", "MAX", "MIN", "STDDEV", "VARIANCE":
		// values[1] it's a field in table
		v := values[1]
		if v != "*" {
			if !ident.IsValid(v) {
				return "", ErrInvalidIdentifier
			}
			q, _ := ident.Quote(v)
			values[1] = q
		}
		groupFuncSQL = fmt.Sprintf(`%s(%s)`, groupFunc, values[1])
		if len(values) == 3 {
			alias := values[2]
			// alias must be a simple identifier (no dot)
			if !ident.IsValid(alias) || strings.Contains(alias, ".") {
				return "", ErrInvalidIdentifier
			}
			groupFuncSQL = fmt.Sprintf(`%s AS "%s"`, groupFuncSQL, alias)
		}
		return
	default:
		err = errors.Wrapf(ErrInvalidGroupFn, "%s", groupFunc)
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

// ShowTable shows table structure
func (adapter *Postgres) ShowTable(schema, table string) adapters.Scanner {
	query := `SELECT table_schema, table_name, ordinal_position as position, column_name,data_type,
			  	CASE WHEN character_maximum_length is not null
					THEN character_maximum_length
					ELSE numeric_precision end as max_length,
			  	is_nullable,
			  	is_generated,
			  	is_updatable,
			  	column_default as default_value
			 FROM information_schema.columns
			 WHERE table_name=$1 AND table_schema=$2
			 ORDER BY table_schema, table_name, ordinal_position`
	return adapter.Query(query, table, schema)
}

// ShowTableCtx shows table structure
func (adapter *Postgres) ShowTableCtx(ctx context.Context, schema, table string) adapters.Scanner {
	query := `SELECT table_schema, table_name, ordinal_position as position, column_name,data_type,
			  	CASE WHEN character_maximum_length is not null
					THEN character_maximum_length
					ELSE numeric_precision end as max_length,
			  	is_nullable,
			  	is_generated,
			  	is_updatable,
			  	column_default as default_value
			 FROM information_schema.columns
			 WHERE table_name=$1 AND table_schema=$2
			 ORDER BY table_schema, table_name, ordinal_position`
	return adapter.QueryCtx(ctx, query, table, schema)
}

// GetDatabase returns the current DB name
func (adapter *Postgres) GetDatabase() string {
	return connection.GetDatabase()
}

// getDBFromCtx tries to get the DB from context adding it to the pool if not
// present, unless DB name is unset in the context - it will then fallback to
// the current DB has been set via `SetDatabase(...)`
func getDBFromCtx(ctx context.Context) (db *sqlx.DB, err error) {
	dbName, ok := ctx.Value(pctx.DBNameKey).(string)
	if ok {
		DB, err := connection.GetFromPool(dbName)
		if err == nil {
			return DB, err
		}
		return connection.AddDatabaseToPool(dbName)
	}
	return connection.Get()
}
