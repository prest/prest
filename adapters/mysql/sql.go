package mysql

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/prest/prest/v2/internal/ident"
)

const (
	pageNumberKey   = "_page"
	pageSizeKey     = "_page_size"
	defaultPageSize = 10
)

var removeOperatorRegex = regexp.MustCompile(`\$[a-z]+\.`)

func (a *Adapter) WhereByRequest(r *http.Request, _ int) (string, []interface{}, error) {
	var whereKey []string
	var whereValues []interface{}
	var orClauses []string
	pid := 0
	for key, val := range r.URL.Query() {
		if !strings.HasPrefix(key, "_") {
			for _, v := range val {
				k, vls, err := whereKeyAndValue(key, v, &pid)
				if err != nil {
					return "", nil, err
				}
				if k != "" {
					whereKey = append(whereKey, k)
					whereValues = append(whereValues, vls...)
				}
			}
		} else if key == "_or" {
			for _, v := range val {
				for _, part := range splitTopLevelOrGroup(v) {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					pos := strings.Index(part, "=")
					if pos <= 0 {
						continue
					}
					k, vls, err := whereKeyAndValue(part[:pos], part[pos+1:], &pid)
					if err != nil {
						return "", nil, err
					}
					if k != "" {
						orClauses = append(orClauses, k)
						whereValues = append(whereValues, vls...)
					}
				}
			}
		}
	}
	if len(orClauses) > 0 {
		whereKey = append(whereKey, "("+strings.Join(orClauses, " OR ")+")")
	}
	where := strings.Join(whereKey, " AND ")
	return where, whereValues, nil
}

func whereKeyAndValue(rawKey, v string, pid *int) (string, []interface{}, error) {
	if v == "" {
		return "", nil, errInvalidOperator
	}
	op := strings.ReplaceAll(removeOperatorRegex.FindString(v), ".", "")
	if op == "" {
		op = "$eq"
	}
	value := removeOperatorRegex.ReplaceAllString(v, "")
	keyInfo := strings.Split(rawKey, ":")
	if len(keyInfo) > 1 {
		switch keyInfo[1] {
		case "jsonb":
			return jsonFilter(keyInfo[0], op, value, pid)
		case "tsquery", "vecdist":
			return "", nil, errUnsupported
		default:
			if !ident.IsValid(keyInfo[0]) {
				return "", nil, errors.Wrapf(errInvalidIdentifier, "%s", keyInfo[0])
			}
			return "", nil, errors.Errorf("unknown type suffix: %s", keyInfo[1])
		}
	}
	if !ident.IsValid(rawKey) {
		return "", nil, errors.Wrapf(errInvalidIdentifier, "%s", rawKey)
	}
	quoted, err := quoteIdent(rawKey)
	if err != nil {
		return "", nil, err
	}
	return applyOperator(quoted, op, value, pid)
}

func jsonFilter(field, op, value string, pid *int) (string, []interface{}, error) {
	parts := strings.Split(field, "->>")
	if len(parts) != 2 || !ident.IsValid(parts[0]) || !ident.IsValid(parts[1]) {
		return "", nil, errors.Wrapf(errInvalidIdentifier, "%v", parts)
	}
	col, err := quoteIdent(parts[0])
	if err != nil {
		return "", nil, err
	}
	attr := strings.ReplaceAll(parts[1], "'", "''")
	left := fmt.Sprintf("%s->>'$.%s'", col, attr)
	return applyOperator(left, op, value, pid)
}

func applyOperator(left, rawOp, value string, pid *int) (string, []interface{}, error) {
	op, err := queryOperator(rawOp)
	if err != nil {
		return "", nil, err
	}
	switch op {
	case "IN", "NOT IN":
		items := strings.Split(value, ",")
		ph := make([]string, len(items))
		vals := make([]interface{}, len(items))
		for i, item := range items {
			vals[i] = item
			ph[i] = "?"
		}
		*pid += len(items)
		return fmt.Sprintf("%s %s (%s)", left, op, strings.Join(ph, ",")), vals, nil
	case "ANY", "SOME":
		items := strings.Split(value, ",")
		ph := make([]string, len(items))
		vals := make([]interface{}, len(items))
		for i, item := range items {
			vals[i] = item
			ph[i] = "?"
		}
		*pid += len(items)
		return fmt.Sprintf("%s IN (%s)", left, strings.Join(ph, ",")), vals, nil
	case "IS NULL", "IS NOT NULL", "IS TRUE", "IS NOT TRUE", "IS FALSE", "IS NOT FALSE":
		return left + " " + op, nil, nil
	default:
		*pid++
		return fmt.Sprintf("%s %s ?", left, op), []interface{}{value}, nil
	}
}

func queryOperator(op string) (string, error) {
	op = strings.ReplaceAll(op, "$", "")
	op = strings.ReplaceAll(op, " ", "")
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
	case "like", "ilike":
		return "LIKE", nil
	case "nlike", "nilike":
		return "NOT LIKE", nil
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
	case "all", "tsquery", "vecdist", "ltreelanc", "ltreerdesc", "ltreematch", "ltreematchtxt":
		return "", errUnsupported
	default:
		return "", errInvalidOperator
	}
}

func (a *Adapter) ReturningByRequest(r *http.Request) (string, error) {
	queries := r.URL.Query()["_returning"]
	if len(queries) == 0 {
		return "", nil
	}
	cols := make([]string, 0, len(queries))
	for _, q := range queries {
		if q == "*" {
			cols = append(cols, "*")
			continue
		}
		quoted, err := quoteIdent(q)
		if err != nil {
			return "", errors.Wrap(errInvalidIdentifier, "Returning")
		}
		cols = append(cols, quoted)
	}
	return strings.Join(cols, ", "), nil
}

func (a *Adapter) SetByRequest(r *http.Request, _ int) (string, []interface{}, error) {
	body := make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", nil, err
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return "", nil, errBodyEmpty
	}
	keys := sortedKeys(body)
	fields := make([]string, 0, len(keys))
	values := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		if !ident.IsValid(key) {
			return "", nil, errors.Wrap(errInvalidIdentifier, "Set")
		}
		quoted, err := quoteIdent(key)
		if err != nil {
			return "", nil, err
		}
		bound, err := bindValue(body[key])
		if err != nil {
			return "", nil, err
		}
		fields = append(fields, quoted+"=?")
		values = append(values, bound)
	}
	return strings.Join(fields, ", "), values, nil
}

func (a *Adapter) ParseInsertRequest(r *http.Request) (string, string, []interface{}, error) {
	body := make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", "", nil, err
	}
	defer closer(r.Body)
	if len(body) == 0 {
		return "", "", nil, errBodyEmpty
	}
	keys := sortedKeys(body)
	fields := make([]string, 0, len(keys))
	values := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		if !ident.IsValid(key) {
			return "", "", nil, errors.Wrap(errInvalidIdentifier, "Insert")
		}
		quoted, err := quoteIdent(key)
		if err != nil {
			return "", "", nil, err
		}
		bound, err := bindValue(body[key])
		if err != nil {
			return "", "", nil, err
		}
		fields = append(fields, quoted)
		values = append(values, bound)
	}
	return strings.Join(fields, ", "), placeholders(1, len(values)), values, nil
}

func (a *Adapter) ParseBatchInsertRequest(r *http.Request) (string, string, []interface{}, error) {
	recordSet := make([]map[string]interface{}, 0)
	if err := json.NewDecoder(r.Body).Decode(&recordSet); err != nil {
		return "", "", nil, err
	}
	defer closer(r.Body)
	if len(recordSet) == 0 {
		return "", "", nil, errBodyEmpty
	}
	keys := sortedKeys(recordSet[0])
	quoted := make([]string, len(keys))
	for i, key := range keys {
		if !ident.IsValid(key) {
			return "", "", nil, errors.Wrap(errInvalidIdentifier, "Insert")
		}
		q, err := quoteIdent(key)
		if err != nil {
			return "", "", nil, err
		}
		quoted[i] = q
	}
	values := make([]interface{}, 0, len(recordSet)*len(keys))
	rows := make([]string, 0, len(recordSet))
	for _, record := range recordSet {
		start := len(values) + 1
		for _, key := range keys {
			bound, err := bindValue(record[key])
			if err != nil {
				return "", "", nil, err
			}
			values = append(values, bound)
		}
		rows = append(rows, placeholders(start, len(values)))
	}
	return strings.Join(quoted, ","), strings.Join(rows, ","), values, nil
}

func bindValue(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return value, nil
		}
		return json.Marshal(value)
	case reflect.Array, reflect.Map:
		return json.Marshal(value)
	default:
		return value, nil
	}
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func closer(body io.Closer) {
	if err := body.Close(); err != nil {
		slog.Error("close body", "err", err)
	}
}

func (a *Adapter) SelectFields(fields []string) (string, error) {
	if len(fields) == 0 {
		return "", errMustSelectOneField
	}
	aux := make([]string, len(fields))
	for i, field := range fields {
		q, err := sanitizeSelectField(field)
		if err != nil {
			return "", err
		}
		aux[i] = q
	}
	return "SELECT " + strings.Join(aux, ",") + " FROM", nil
}

func (a *Adapter) SelectSQL(selectStr, database, schema, table string) string {
	return selectStr + " " + tableReference(schema, table)
}

func (a *Adapter) InsertSQL(database, schema, table, names, placeholders string) string {
	return fmt.Sprintf("INSERT INTO %s(%s) VALUES%s", tableReference(schema, table), names, placeholders)
}

func (a *Adapter) UpdateSQL(database, schema, table, setSyntax string) string {
	return fmt.Sprintf("UPDATE %s SET %s", tableReference(schema, table), setSyntax)
}

func (a *Adapter) DeleteSQL(database, schema, table string) string {
	return fmt.Sprintf("DELETE FROM %s", tableReference(schema, table))
}

func tableReference(schema, table string) string {
	return mustQuote(schema) + "." + mustQuote(table)
}

func (a *Adapter) OrderByRequest(r *http.Request) (string, error) {
	queries := r.URL.Query()
	if queries.Get("_korder") != "" {
		return "", errUnsupported
	}
	reqOrder := queries.Get("_order")
	if reqOrder == "" {
		return "", nil
	}
	terms := make([]string, 0)
	for _, fld := range strings.Split(reqOrder, ",") {
		desc := false
		field := fld
		if strings.HasPrefix(field, "-") {
			desc = true
			field = field[1:]
		}
		if !ident.IsValid(field) {
			return "", errInvalidIdentifier
		}
		q, err := quoteIdent(field)
		if err != nil {
			return "", err
		}
		if desc {
			q += " DESC"
		}
		terms = append(terms, q)
	}
	return " ORDER BY " + strings.Join(terms, " , "), nil
}

func (a *Adapter) PaginateIfPossible(r *http.Request) (string, error) {
	values := r.URL.Query()
	raw, ok := values[pageNumberKey]
	if !ok || len(raw) == 0 {
		return "", nil
	}
	pageNumber, err := strconv.Atoi(raw[0])
	if err != nil {
		return "", err
	}
	pageSize := defaultPageSize
	if size, ok := values[pageSizeKey]; ok && len(size) > 0 {
		pageSize, err = strconv.Atoi(size[0])
		if err != nil {
			return "", err
		}
	}
	if pageNumber < 1 {
		pageNumber = 1
	}
	// MySQL rejects OFFSET(page-1)*size. Emit the same LIMIT n OFFSET m shape with the offset evaluated.
	offset := (pageNumber - 1) * pageSize
	return fmt.Sprintf("LIMIT %d OFFSET %d", pageSize, offset), nil
}

func (a *Adapter) DistinctClause(r *http.Request) (string, error) {
	if r.URL.Query().Get("_distinct") == "true" {
		return "SELECT DISTINCT", nil
	}
	return "", nil
}

func (a *Adapter) CountByRequest(req *http.Request) (string, error) {
	queries := req.URL.Query()
	countFields := queries.Get("_count")
	if countFields == "" {
		return "", nil
	}
	selectFields := ""
	if raw := queries.Get("_select"); raw != "" {
		parts := strings.Split(raw, ",")
		for i, p := range parts {
			s, err := sanitizeSelectField(strings.TrimSpace(p))
			if err != nil {
				return "", errInvalidIdentifier
			}
			parts[i] = s
		}
		selectFields = ", " + strings.Join(parts, ",")
	}
	fields := strings.Split(countFields, ",")
	for i, field := range fields {
		if field != "*" && !ident.IsValid(field) {
			return "", errInvalidIdentifier
		}
		if field != "*" {
			q, err := quoteIdent(field)
			if err != nil {
				return "", err
			}
			fields[i] = q
		}
	}
	return fmt.Sprintf("SELECT COUNT(%s)%s FROM", strings.Join(fields, ","), selectFields), nil
}

func (a *Adapter) TimeBucketClause(*http.Request) (string, error) { return "", nil }

func (a *Adapter) JoinByRequest(r *http.Request) ([]string, error) {
	joined := map[string]bool{}
	var values []string
	for _, clause := range r.URL.Query()["_join"] {
		if clause == "" {
			continue
		}
		joinQuery, exposed, err := joinClauseSQL(clause)
		if err != nil {
			return nil, err
		}
		if joined[exposed] {
			return nil, errors.Wrapf(errInvalidJoin, "table %q is joined more than once", exposed)
		}
		joined[exposed] = true
		values = append(values, joinQuery)
	}
	return values, nil
}

func joinClauseSQL(clause string) (string, string, error) {
	joinArgs := strings.Split(clause, ":")
	if len(joinArgs) != 5 {
		return "", "", errJoinArgs
	}
	jt := strings.ToUpper(joinArgs[0])
	switch jt {
	case "INNER", "LEFT", "RIGHT", "CROSS":
	case "FULL":
		return "", "", errUnsupported
	default:
		return "", "", errInvalidJoin
	}
	if !ident.IsValid(joinArgs[1]) || !ident.IsValid(joinArgs[2]) || !ident.IsValid(joinArgs[4]) {
		return "", "", errInvalidIdentifier
	}
	op, err := queryOperator(joinArgs[3])
	if err != nil {
		return "", "", err
	}
	if op == "ANY" || op == "SOME" || strings.HasPrefix(op, "IS ") || op == "IN" || op == "NOT IN" {
		return "", "", errInvalidJoin
	}
	exposed := joinArgs[1]
	target := joinArgs[1]
	if parts := strings.Split(joinArgs[1], "."); len(parts) == 2 {
		exposed = parts[1]
		left, err := quoteIdent(parts[0])
		if err != nil {
			return "", "", err
		}
		right, err := quoteIdent(parts[1])
		if err != nil {
			return "", "", err
		}
		target = left + "." + right
	} else {
		q, err := quoteIdent(joinArgs[1])
		if err != nil {
			return "", "", err
		}
		target = q
	}
	left := strings.Split(joinArgs[2], ".")
	right := strings.Split(joinArgs[4], ".")
	if len(left) != 2 || len(right) != 2 {
		return "", "", errInvalidJoin
	}
	lq0, err := quoteIdent(left[0])
	if err != nil {
		return "", "", err
	}
	lq1, err := quoteIdent(left[1])
	if err != nil {
		return "", "", err
	}
	rq0, err := quoteIdent(right[0])
	if err != nil {
		return "", "", err
	}
	rq1, err := quoteIdent(right[1])
	if err != nil {
		return "", "", err
	}
	return fmt.Sprintf(" %s JOIN %s ON %s.%s %s %s.%s ", jt, target, lq0, lq1, op, rq0, rq1), exposed, nil
}

func (a *Adapter) GroupByClause(r *http.Request) string {
	groupQuery := r.URL.Query().Get("_groupby")
	if groupQuery == "" {
		return ""
	}
	if strings.Contains(groupQuery, "->>having") {
		return groupByHaving(groupQuery)
	}
	fields := strings.Split(groupQuery, ",")
	for i, field := range fields {
		field = strings.TrimSpace(field)
		if strings.Contains(field, "(") && strings.Contains(field, ")") {
			if !isSafeSQLExpression(field) {
				return ""
			}
			fields[i] = field
			continue
		}
		if !ident.IsValid(field) {
			return ""
		}
		q, err := quoteIdent(field)
		if err != nil {
			return ""
		}
		fields[i] = q
	}
	return "GROUP BY " + strings.Join(fields, ",")
}

func groupByHaving(groupQuery string) string {
	params := strings.Split(groupQuery, ":")
	groupFieldQuery := strings.Split(groupQuery, "->>having")
	fields := strings.Split(groupFieldQuery[0], ",")
	for i, field := range fields {
		if !ident.IsValid(field) {
			return ""
		}
		q, err := quoteIdent(field)
		if err != nil {
			return ""
		}
		fields[i] = q
	}
	grouped := "GROUP BY " + strings.Join(fields, ",")
	if len(params) != 5 {
		return grouped
	}
	groupFunc, err := normalizeGroupFunction(params[1] + ":" + params[2])
	if err != nil {
		return grouped
	}
	operator, err := queryOperator(params[3])
	if err != nil {
		return grouped
	}
	val := params[4]
	if _, errNum := strconv.ParseFloat(val, 64); errNum != nil {
		val = "'" + strings.ReplaceAll(val, "'", "''") + "'"
	}
	return grouped + " HAVING " + groupFunc + " " + operator + " " + val
}

func sanitizeSelectField(field string) (string, error) {
	if field == "*" {
		return "*", nil
	}
	if prefix, found := strings.CutSuffix(field, ".*"); found {
		q, err := quoteIdent(prefix)
		if err != nil {
			return "", errors.Wrapf(errInvalidIdentifier, "%s", field)
		}
		return q + ".*", nil
	}
	if groupFunc, err := normalizeGroupFunction(field); err == nil && groupFunc != "" {
		return groupFunc, nil
	}
	if quotedAggRegex.MatchString(field) {
		return field, nil
	}
	if !ident.IsValid(field) {
		return "", errors.Wrapf(errInvalidIdentifier, "%s", field)
	}
	return quoteIdent(field)
}

var quotedAggRegex = regexp.MustCompile(
	"^(SUM|AVG|MAX|MIN|STDDEV|VARIANCE)" +
		"\\((\\*|`[A-Za-z_]\\w*`(\\.[A-Za-z_]\\w*`)*)\\)" +
		"( AS `[A-Za-z_]\\w*`)?$")

func normalizeGroupFunction(paramValue string) (string, error) {
	values := strings.Split(paramValue, ":")
	groupFunc := strings.ToUpper(values[0])
	switch groupFunc {
	case "SUM", "AVG", "MAX", "MIN", "STDDEV", "VARIANCE":
		if len(values) < 2 {
			return "", errors.Wrapf(errInvalidGroupFn, "%s", groupFunc)
		}
		v := values[1]
		if v != "*" {
			if !ident.IsValid(v) {
				return "", errInvalidIdentifier
			}
			q, err := quoteIdent(v)
			if err != nil {
				return "", err
			}
			v = q
		}
		sql := fmt.Sprintf("%s(%s)", groupFunc, v)
		if len(values) == 3 {
			alias := values[2]
			if !ident.IsValid(alias) || strings.Contains(alias, ".") {
				return "", errInvalidIdentifier
			}
			sql = fmt.Sprintf("%s AS `%s`", sql, alias)
		}
		return sql, nil
	default:
		return "", errors.Wrapf(errInvalidGroupFn, "%s", groupFunc)
	}
}

var allowedGroupByFunctions = map[string]struct{}{
	"time_bucket": {},
	"date_trunc":  {},
	"extract":     {},
	"upper":       {},
	"lower":       {},
	"length":      {},
	"coalesce":    {},
	"nullif":      {},
	"trim":        {},
	"abs":         {},
	"round":       {},
	"floor":       {},
	"ceil":        {},
}

func isSafeSQLExpression(expr string) bool {
	if strings.Contains(expr, "--") || strings.Contains(expr, ";") || strings.Contains(expr, "/*") {
		return false
	}
	if !strings.HasSuffix(expr, ")") {
		return false
	}
	idx := strings.Index(expr, "(")
	if idx <= 0 {
		return false
	}
	funcName := strings.ToLower(strings.TrimSpace(expr[:idx]))
	if strings.HasPrefix(funcName, "pg_") {
		return false
	}
	if _, ok := allowedGroupByFunctions[funcName]; !ok {
		return false
	}
	args := expr[idx+1 : len(expr)-1]
	if strings.ContainsAny(args, "()") {
		return false
	}
	for _, ch := range args {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
			ch == '_' || ch == ',' || ch == '\'' || ch == ' ' || ch == '.' || ch == '-') {
			return false
		}
	}
	return true
}

func splitTopLevelOrGroup(v string) []string {
	var parts []string
	var current strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		part := strings.TrimSpace(current.String())
		if part != "" {
			parts = append(parts, part)
		}
		current.Reset()
	}
	for i := 0; i < len(v); {
		ch := v[i]
		if inSingle {
			current.WriteByte(ch)
			if ch == '\'' {
				if i+1 < len(v) && v[i+1] == '\'' {
					current.WriteByte(v[i+1])
					i += 2
					continue
				}
				inSingle = false
			}
			i++
			continue
		}
		if inDouble {
			current.WriteByte(ch)
			if ch == '"' {
				if i+1 < len(v) && v[i+1] == '"' {
					current.WriteByte(v[i+1])
					i += 2
					continue
				}
				inDouble = false
			}
			i++
			continue
		}
		if i+1 < len(v) && v[i] == '|' && v[i+1] == '|' {
			flush()
			i += 2
			continue
		}
		if ch == '\'' {
			inSingle = true
			current.WriteByte(ch)
			i++
			continue
		}
		if ch == '"' {
			inDouble = true
			current.WriteByte(ch)
			i++
			continue
		}
		if i+2 < len(v) && strings.EqualFold(v[i:i+2], "OR") &&
			(i == 0 || unicode.IsSpace(rune(v[i-1]))) && unicode.IsSpace(rune(v[i+2])) {
			flush()
			i += 2
			for i < len(v) && unicode.IsSpace(rune(v[i])) {
				i++
			}
			continue
		}
		current.WriteByte(ch)
		i++
	}
	flush()
	return parts
}
