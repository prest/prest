package postgres

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/internal/ident"
	"github.com/prest/prest/v2/internal/postgres/statements"

	"github.com/pkg/errors"
)

// JoinByRequest implements join in queries
func (adapter *postgres) JoinByRequest(r *http.Request) (values []string, err error) {
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
func (adapter *postgres) SelectFields(fields []string) (sql string, err error) {
	if len(fields) == 0 {
		err = ErrMustSelectOneField
		return
	}
	var aux []string

	for _, field := range fields {
		q, ferr := sanitizeSelectField(field)
		if ferr != nil {
			err = ferr
			return
		}
		aux = append(aux, q)
	}
	sql = fmt.Sprintf("SELECT %s FROM", strings.Join(aux, ","))
	return
}

// OrderByRequest implements ORDER BY in queries
func (adapter *postgres) OrderByRequest(r *http.Request) (values string, err error) {
	queries := r.URL.Query()
	reqOrder := queries.Get("_order")
	reqKOrder := queries.Get("_korder")

	if reqOrder == "" && reqKOrder == "" {
		return
	}

	terms := []string{}

	// _korder must lead the ORDER BY so pgvector can use an ANN index for the
	// nearest-neighbor scan; a preceding regular _order term would force an
	// exact sort. e.g. _korder=embedding:l2:[1,2,3] -> "embedding" <-> '[1,2,3]'::vector
	if reqKOrder != "" {
		term, kerr := buildVectorOrderTerm(reqKOrder)
		if kerr != nil {
			err = kerr
			return
		}
		terms = append(terms, term)
	}

	if reqOrder != "" {
		for _, fld := range strings.Split(reqOrder, ",") {
			desc := false
			field := fld
			if strings.HasPrefix(field, "-") {
				desc = true
				field = field[1:]
			}
			if !ident.IsValid(field) {
				err = ErrInvalidIdentifier
				return
			}
			q, _ := ident.Quote(field)
			if desc {
				q = fmt.Sprintf("%s DESC", q)
			}
			terms = append(terms, q)
		}
	}

	values = " ORDER BY " + strings.Join(terms, " , ")
	return
}

// CountByRequest implements COUNT(fields) OPERTATION
func (adapter *postgres) CountByRequest(req *http.Request) (countQuery string, err error) {
	queries := req.URL.Query()
	countFields := queries.Get("_count")
	selectFields := queries.Get("_select")
	if countFields == "" {
		return
	}
	if selectFields != "" {
		parts := strings.Split(selectFields, ",")
		for i, p := range parts {
			s, ferr := sanitizeSelectField(strings.TrimSpace(p))
			if ferr != nil {
				err = ErrInvalidIdentifier
				return
			}
			parts[i] = s
		}
		selectFields = fmt.Sprintf(", %s", strings.Join(parts, ","))
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

// DistinctClause get params in request to add distinct clause
func (adapter *postgres) DistinctClause(r *http.Request) (distinctQuery string, err error) {
	queries := r.URL.Query()
	checkQuery := queries.Get("_distinct")
	distinctQuery = ""

	if checkQuery == "true" {
		distinctQuery = "SELECT DISTINCT"
	}
	return
}

// GroupByClause get params in request to add group by clause
func (adapter *postgres) GroupByClause(r *http.Request) (groupBySQL string) {
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
		field = strings.TrimSpace(field)
		// Handle function calls (e.g., time_bucket('1 minute', time)) for TimescaleDB support.
		// If field contains parentheses, treat it as a raw SQL expression (must already be safe).
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
		q, _ := ident.Quote(field)
		fields[i] = q
	}
	groupQuery = strings.Join(fields, ",")
	groupBySQL = fmt.Sprintf(statements.GroupBy, groupQuery)
	return
}

// TimeBucketClause is not supported in the base postgres adapter.
// This is a TimescaleDB-specific feature; the TimescaleDB adapter overrides this method.
func (adapter *postgres) TimeBucketClause(r *http.Request) (groupBySQL string, err error) {
	return
}

// allowedGroupByFunctions is the explicit allowlist for function expressions in _groupby.
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

// isSafeSQLExpression validates that a SQL expression used in GROUP BY is an
// allowlisted function call with safe characters (no comments, no pg_* funcs).
func isSafeSQLExpression(expr string) bool {
	if strings.Contains(expr, "--") || strings.Contains(expr, ";") || strings.Contains(expr, "/*") {
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

	for _, ch := range expr {
		if !((ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '_' || ch == '(' || ch == ')' || ch == ',' ||
			ch == '\'' || ch == ' ' || ch == '.' || ch == '-') {
			return false
		}
	}

	balance := 0
	for _, ch := range expr {
		if ch == '(' {
			balance++
		} else if ch == ')' {
			balance--
		}
		if balance < 0 {
			return false
		}
	}
	return balance == 0
}

// quotedAggRegex matches a NormalizeGroupFunction-produced aggregate expression:
// FUNC("ident") or FUNC("a"."b") with an optional  AS "alias". Anchored and strict:
// only the six aggregate functions, quoted simple identifiers (or *), no subselects,
// no extra parens/spaces. Rejects (SELECT ...)"x", pg_read_file(...)"f", etc.
var quotedAggRegex = regexp.MustCompile(
	`^(SUM|AVG|MAX|MIN|STDDEV|VARIANCE)` +
		`\((\*|"[A-Za-z_]\w*"(\."[A-Za-z_]\w*")*)\)` +
		`( AS "[A-Za-z_]\w*")?$`)

// sanitizeSelectField returns the safe SQL form of one _select field, or an error
// if it is neither "*", a valid identifier, nor a whitelisted aggregate expression.
// It is the single validation gate shared by SelectFields and CountByRequest so that
// no attacker-controlled _select value ever reaches raw SQL concatenation.
func sanitizeSelectField(field string) (string, error) {
	if field == "*" {
		return "*", nil
	}
	if groupFunc, _ := NormalizeGroupFunction(field); groupFunc != "" {
		return groupFunc, nil // colon-syntax: already validated + quoted
	}
	if quotedAggRegex.MatchString(field) {
		return field, nil // pre-quoted aggregate from the _groupby path
	}
	if !ident.IsValid(field) {
		return "", errors.Wrapf(ErrInvalidIdentifier, "%s", field)
	}
	q, _ := ident.Quote(field)
	return q, nil
}

// NormalizeGroupFunction normalize url params values to sql group functions
func NormalizeGroupFunction(paramValue string) (groupFuncSQL string, err error) {
	values := strings.Split(paramValue, ":")
	groupFunc := strings.ToUpper(values[0])
	switch groupFunc {
	case "SUM", "AVG", "MAX", "MIN", "STDDEV", "VARIANCE":
		// A bare aggregate keyword (no ":field") is not a group function; reject
		// it here so callers treat it as an ordinary field instead of panicking.
		if len(values) < 2 {
			err = errors.Wrapf(ErrInvalidGroupFn, "%s", groupFunc)
			return
		}
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
